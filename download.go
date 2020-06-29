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
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/schollz/progressbar"
)

func getPlaylist(u string) error {
	url, err := url.Parse(u)
	if err != nil {
		return err
	}

	url.Path = filepath.Dir(url.Path)

	base := url.String() + "/"

	resp, err := http.Get(u)
	if err != nil {
		return err
	}

	body := resp.Body
	defer body.Close()

	lines := make([]string, 0)

	s := bufio.NewScanner(body)
	for s.Scan() {
		t := s.Text()
		if !strings.HasPrefix(t, "#") {
			lines = append(lines, base+t)
		}
	}

	var wg sync.WaitGroup

	// канал для создания ограничение на одовременно запускаемые рутины
	ch := make(chan int, 30)

	// прогрессбар на скачивание
	pb := progressbar.New(len(lines))
	pb.Describe("Скачивание сегментов")
	pb.RenderBlank()

	// запускаем рутину на каждый сегмент
	for i := 0; i < len(lines); i++ {
		wg.Add(1)
		// пишем в буферизованный канал для ограничения одновременно запускаемых рутин
		ch <- i
		// запуск скачивания
		go downloadSegment(lines[i], i, ch, &wg, pb)
	}

	// ожидаем, пока все закачки закончатся
	wg.Wait()
	pb.Finish()

	return nil
}

func getSegments(s string, n int) {
	// создаём темплейт из заданного url
	t := template.New("")
	t.Parse(s)

	var wg sync.WaitGroup

	// канал для создания ограничение на одовременно запускаемые рутины
	ch := make(chan int, 30)

	// прогрессбар на скачивание
	pb := progressbar.New(n)
	pb.Describe("Скачивание сегментов")
	pb.RenderBlank()

	// запускаем рутину на каждый сегмент
	for i := 1; i <= n; i++ {
		// буфер для создания строки
		var b bytes.Buffer
		// создаём новый url из темплейта
		t.Execute(&b, i)

		wg.Add(1)
		// пишем в буферизованный канал для ограничения одновременно запускаемых рутин
		ch <- i
		// запуск скачивания
		go downloadSegment(b.String(), i, ch, &wg, pb)
	}

	// ожидаем, пока все закачки закончатся
	wg.Wait()
	pb.Finish()
}

func downloadSegment(u string, i int, ch chan int, wg *sync.WaitGroup, pb *progressbar.ProgressBar) {
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
	fn := strconv.Itoa(i) + ".ts"

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
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("%v, файл: %v\n", err, fn)
	}

	defer f.Close()

	_, err = buf.WriteTo(f)
	if err != nil {
		log.Printf("Ошибка записи файла %v: %v", fn, err)
	}
}
