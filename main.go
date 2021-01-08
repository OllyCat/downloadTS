package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	var con bool
	var n int
	var u, s string

	// флаги запуска
	pflag.IntVarP(&n, "number", "n", 1, "количество сегментов")
	pflag.BoolVarP(&con, "concat-only", "c", false, "только объединение сегментов")
	pflag.StringVarP(&u, "url", "u", "", "url плейлиста m3u8")
	pflag.StringVarP(&s, "seg-template", "s", "", "url-темплейт, на месте числа надо поставить {{.}}")

	// парсим флаги
	pflag.Parse()

	if !con {
		log.Printf("%#v\n\n", s)
		switch {
		case !strings.HasPrefix(s, "http"): //&& !strings.HasPrefix(u, "http"):
			fmt.Println("Invalid URL")
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
			pflag.Usage()
			os.Exit(1)
		}
	}

	err := concatTs()
	if err != nil {
		log.Printf("Ошибка сбора финального файла: %v\n", err)
	}
}
