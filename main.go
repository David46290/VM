package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
)

var (
	target_folder      = "images"
	supported_img_type = [2]string{"jpeg", "png"}
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

func image_type(img image.Image) (int8, error) {
	switch typedImg := img.(type) {
	case *image.YCbCr:
		fmt.Printf("YCbCr planes: Y=%d, Cb=%d, Cr=%d\n",
			len(typedImg.Y), len(typedImg.Cb), len(typedImg.Cr))
		fmt.Printf("subsample ratio: %v\n", typedImg.SubsampleRatio)
		return 3, nil
	case *image.RGBA:
		fmt.Println("RGBA storage: 4 channels")
		return 4, nil
	case *image.NRGBA:
		fmt.Println("NRGBA storage: 4 channels")
		return 4, nil
	case *image.Gray:
		fmt.Println("Gray storage: 1 channel")
		return 1, nil
	default:
		fmt.Printf("Other image representation: %T\n", typedImg)
		return 0, fmt.Errorf("unsupported image representation: %T", typedImg)
	}

}

func image_detail(img image.Image, format string) (int, int, string, error) {
	// get width, height, histogram
	if !slices.Contains(supported_img_type[:], format) {
		return 0, 0, "", fmt.Errorf("unsupported image format: %s\n", format)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	num_channels, err := image_type(img)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error determining image type: %v\n", err)
	}
	fmt.Printf("Number of channel of image: %d\n", num_channels)
	for idx_x := 0; idx_x < width; idx_x++ {
		for idx_y := 0; idx_y < height; idx_y++ {
			_ = img.At(idx_x, idx_y)
		}
	}
	return width, height, "histogram", nil
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
		width, height, histo, err := image_detail(img, format)
		if err != nil {
			fmt.Printf("Error processing image %s: %v\n", name, err)
			continue
		}
		fmt.Printf("%s: %s, %dx%d, %s\n", name, format, width, height, histo)
		entry_count++
	}
	fmt.Printf("Number of images in target folder: %d\n", entry_count)
}
