package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	var con bool
	var n int
	var u, s string

	// флаги запуска
	flag.IntVar(&n, "n", 0, "количество сегментов")
	flag.BoolVar(&con, "c", false, "только объединение сегментов")
	flag.StringVar(&u, "url", "", "url плейлиста m3u8")
	flag.StringVar(&s, "seg", "", "url-темплейт, на месте числа надо поставить {{.}}")

	// парсим флаги
	flag.Parse()

	if !con {
		switch {
		case !strings.HasPrefix(s, "http") && !strings.HasPrefix(u, "http"):
			fmt.Println("Invalod URL")
			flag.Usage()
			os.Exit(1)
		case s != "" && n != 0:
			getSegments(s, n)
		case u != "":
			err := getPlaylist(u)
			if err != nil {
				fmt.Printf("Ошибка загрузки плейлиста: %v\n", err)
				os.Exit(1)
			}
		default:
			flag.Usage()
			os.Exit(1)
		}
	}

	err := concatTs()
	if err != nil {
		log.Printf("Ошибка сбора финального файла: %v\n", err)
	}
}
