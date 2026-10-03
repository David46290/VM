package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
)

var (
	target_folder = "images"
)

func validate_file(path string) (image.Image, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}

func main() {
	current_dir, _ := os.Getwd()
	target_dir, _ := filepath.Abs(target_folder)
	fmt.Printf("Current working directory: %s\n", current_dir)
	fmt.Printf("Target folder: %s\n", target_dir)

	entries, err := os.ReadDir(target_dir)
	if err != nil {
		log.Fatalf("Can't read target folder: %v\n", err)
	}
	entry_count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(target_dir, name)
		img, format, err := validate_file(path)
		if err != nil {
			fmt.Printf("File %s is not a valid image.\n", name)
			continue
		}

		entry_count++
	}
	fmt.Printf("Number of images in target folder: %d\n", entry_count)
}
