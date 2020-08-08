package main

import (
	"bufio"
	"bytes"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/schollz/progressbar"
)

func getPlaylist(u string) error {
	// парсим url
	url, err := url.Parse(u)
	if err != nil {
		return err
	}

	// берём только путь до файлов
	url.Path = filepath.Dir(url.Path)
	base := url.String() + "/"

	// качаем m3u8
	resp, err := http.Get(u)
	if err != nil {
		return err
	}

	body := resp.Body
	defer body.Close()

	// создаём пустой список для сегментов
	lines := make([]string, 0)

	// читаем файл
	s := bufio.NewScanner(body)
	for s.Scan() {
		t := s.Text()
		// если не комментарий, то добавляем, прибавив url
		if !strings.HasPrefix(t, "#") {
			lines = append(lines, base+t)
		}
	}

	// запускаем скачивание
	download(lines)

	return nil
}

func getSegments(s string, n int) {
	// создаём темплейт из заданного url
	t := template.New("")
	t.Parse(s)

	// создаём пустой список
	lines := make([]string, 0)

	// заполняем его из темплейта
	for i := 1; i <= n; i++ {
		var b bytes.Buffer
		t.Execute(&b, i)
		lines = append(lines, b.String())
	}

	// запускаем закачку
	download(lines)
}

func download(l []string) {
	// основная функция загрузчика
	var wg sync.WaitGroup

	// канал для создания ограничение на одовременно запускаемые рутины
	ch := make(chan int, 30)

	// прогрессбар на скачивание
	pb := progressbar.New(len(l))
	pb.Describe("Скачивание сегментов")
	pb.RenderBlank()

	// запускаем рутину на каждый сегмент
	for i := 0; i < len(l); i++ {
		wg.Add(1)
		// пишем в буферизованный канал для ограничения одновременно запускаемых рутин
		ch <- i
		// запуск скачивания
		go downloadSegment(l[i], ch, &wg, pb)
	}

	// ожидаем, пока все закачки закончатся
	wg.Wait()
	pb.Finish()
}

func downloadSegment(u string, ch chan int, wg *sync.WaitGroup, pb *progressbar.ProgressBar) {
	// функция загрузки одного сегмента
	// отложенные вызовы wg
	defer wg.Done()
	// и функция чтения из буферизованного канала, что бы по окончании сказчивния освободить очередь
	defer func() {
		_ = <-ch
		// обновить прогрессбар
		pb.Add(1)
	}()

	// ответ сервера
	var resp *http.Response
	// ошибка
	var err error
	// счётчик ошибок скачивания
	var c int
	// буфер для скачиваемых данных
	var buf bytes.Buffer

	// инициализируем сид для случайной задержки, что бы сервер нас не отфутболил от большого количества запросов
	rand.Seed(time.Now().UnixNano())

	// имя выходного файла
	fn := filepath.Base(u)

	// цикл в сто повторов
	for c < 100 {
		// случайная задержка до 6 секунд
		rnd := rand.Intn(6)
		time.Sleep(time.Duration(rnd) * time.Second)

		// запрос к серверу
		resp, err = http.Get(u)
		if err != nil {
			// если ошибка увеличиваем счётчик ошибок и возомновим цикл сначала
			log.Printf("Ошибка: %v, URL: %v\n", err, u)
			c++
			continue
		}

		// получаем тело
		body := resp.Body
		defer body.Close()

		// копируем данные
		_, err := buf.ReadFrom(body)
		// если ошибка - закроем файл, увеличим счётчик ошибок и начнём цикл заново
		if err == nil {
			// если всё хорошо - закончим
			break
		}
		//log.Printf("Сбой скачивания: %v\n", err)
		c++
		buf.Reset()
	}
	// если количество повторов зашкалило - сообщим об этом
	if c >= 100 {
		log.Printf("Количество повторных запросов для файла %v превысило предел.\n", fn)
		return
	}

	// создаём файл для записи
	f, err := os.Create(fn)
	if err != nil {
		log.Printf("%v, файл: %v\n", err, fn)
	}

	defer f.Close()

	_, err = buf.WriteTo(f)
	if err != nil {
		log.Printf("Ошибка записи файла %v: %v", fn, err)
	}
}
