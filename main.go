package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"text/template"
	"time"
)

func main() {
	var n int
	var s string

	// флаги запуска
	flag.IntVar(&n, "n", 0, "количество сегментов")
	flag.StringVar(&s, "url", "", "url-темплейт, на месте числа надо поставить {{.}}")

	// парсим флаги
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// создаём темплейт из заданного url
	t := template.New("")
	t.Parse(s)

	var wg sync.WaitGroup

	// канал для создания ограничение на одовременно запускаемые рутины
	ch := make(chan int, 30)

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
		go getSegment(b.String(), i, ch, &wg)
	}

	// ожидаем, пока все закачки закончатся
	wg.Wait()

	// создаём общий файл
	fo, err := os.OpenFile("f.mp4", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// цикл по всем сегментам
	for i := 1; i <= n; i++ {
		// имя файла сегмента
		fn := strconv.Itoa(i) + ".ts"
		// открываем файл на чтение
		fi, err := os.Open(fn)
		if err != nil {
			log.Fatal(err)
		}

		// компируем сегмент в общий файл
		n, err := io.Copy(fo, fi)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Добавлено в финальный файл %d-я часть. %v байт. Результат: %v\n", i, n, err)
		// закрываем файл сегмента
		fi.Close()
		// удаляем его
		os.Remove(fn)
	}

	// закрываем общий файл
	fo.Close()

	// проходим по файлу ffmpeg для исправления косяков в потоках
	c := exec.Command("ffmpeg", "-i", "f.mp4", "-codec", "copy", "final.mp4")
	c.Run()
	// удаляем общий файл
	os.Remove("f.mp4")
}

// Функция закачки сегмента
func getSegment(u string, i int, ch chan int, wg *sync.WaitGroup) {
	// отложенные вызовы wg
	defer wg.Done()
	// и функция чтения из буферизованного канала, что бы по окончании сказчивния освободить очередь
	defer func() {
		_ = <-ch
	}()

	// ответ сервера
	var resp *http.Response
	// ошибка
	var err error
	// счётчик ошибок скачивания
	var c int

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
		b := resp.Body
		defer b.Close()

		// создаём файл для записи
		f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("%v, файл: %v\n", err, fn)
			return
		}
		defer f.Close()

		// копируем данные
		n, err := io.Copy(f, b)
		// если ошибка - закроем файл, увеличим счётчик ошибок и начнём цикл заново
		if err != nil {
			//log.Printf("Сбой скачивания: %v\n", err)
			f.Close()
			c++
			continue
		}
		// если всё хорошо - сообщим и закончим
		log.Printf("Сохранено %v байт в файл %v. Результат копирования: %v\n", n, fn, err)
		break
	}
	// если количество повторов зашкалило - сообщим об этом
	if c >= 100 {
		log.Printf("Количество повторных запросов для файла %v превысило предел.\n", fn)
	}
}
