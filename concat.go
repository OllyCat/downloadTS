package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/facette/natsort"
	"github.com/schollz/progressbar/v3"
)

func concatTs() error {
	fList, err := filepath.Glob("*.ts")
	natsort.Sort(fList)

	if err != nil {
		return err
	}

	// создаём общий файл
	fo, err := os.Create("f.mp4")
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()

	// прогрессбар для финального файла
	pb := progressbar.New(len(fList))
	pb.Describe("Сборка сегментов в файл")
	pb.Clear()
	pb.RenderBlank()

	// цикл по всем сегментам
	for _, fn := range fList {
		// открываем файл на чтение
		fi, err := os.Open(fn)
		if err != nil {
			log.Fatal(err)
		}

		// компируем сегмент в общий файл
		_, err = io.Copy(fo, fi)
		if err != nil {
			return err
		}

		// закрываем файл сегмента
		fi.Close()
		// удаляем его
		os.Remove(fn)
		// обновляем програссбар
		pb.Add(1)
	}

	// проходим по файлу ffmpeg для исправления косяков в потоках
	c := exec.Command("ffmpeg", "-i", "f.mp4", "-codec", "copy", "final.mp4")
	err = c.Run()
	if err != nil {
		return err
	}
	// удаляем общий файл
	err = os.Remove("f.mp4")
	if err != nil {
		return err
	}
	return nil
}
