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
	target_folder         = "images"
	supported_img_type    = [2]string{"jpeg", "png"}
	histo_folder          = "output/debug/histogram"
	channel_bit           = 4             // bit depth for each channel (default 4 for histogram generation)
	channel_bit_threshold = [2]int{1, 10} //max 10 to prevent overflow of uint32 in histogram key
	default_channel_bit   = 16
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

func map_sort_keys(m map[uint32]int) ([]uint32, error) {
	keys := make([]uint32, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, nil
}

func hist_count_inspect(histogram map[uint32]int, bounds image.Rectangle) error {
	sum := 0
	for _, count := range histogram {
		sum += count
	}

	imh_height := bounds.Max.Y - bounds.Min.Y
	imh_width := bounds.Max.X - bounds.Min.X
	expected_count := imh_height * imh_width
	if sum != expected_count {
		return fmt.Errorf("histogram count mismatch: expected %d, got %d", expected_count, sum)
	}
	return nil
}

func img_histogram(img image.Image) (map[uint32]int, []uint32, error) {
	histogram := make(map[uint32]int)
	bounds := img.Bounds()
	for idx_x := bounds.Min.X; idx_x < bounds.Max.X; idx_x++ {
		for idx_y := bounds.Min.Y; idx_y < bounds.Max.Y; idx_y++ {
			r, g, b, _ := img.At(idx_x, idx_y).RGBA()
			// Convert to 8-bit values
			r_shrink := r >> (default_channel_bit - channel_bit)
			g_shrink := g >> (default_channel_bit - channel_bit)
			b_shrink := b >> (default_channel_bit - channel_bit)
			// Create a simple histogram key (you might want a more sophisticated approach)
			key := (r_shrink << (2 * channel_bit)) | (g_shrink << channel_bit) | b_shrink
			histogram[key]++

		}
	}
	if err := hist_count_inspect(histogram, bounds); err != nil {
		return nil, nil, err
	}
	sorted_keys, err := map_sort_keys(histogram)
	if err != nil {
		return nil, nil, err
	}

	return histogram, sorted_keys, nil
}

func image_detail(img image.Image, format string) (map[uint32]int, []uint32, error) {
	// get width, height, histogram
	if !slices.Contains(supported_img_type[:], format) {
		return nil, nil, fmt.Errorf("unsupported image format: %s\n", format)
	}
	histogram, hist_keys_sorted, err := img_histogram(img)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating histogram: %v\n", err)
	}
	return histogram, hist_keys_sorted, nil
}

func save_histogram(histogram map[uint32]int, hist_keys_sorted []uint32, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating histogram file: %v\n", err)
	}
	defer file.Close()

	hexWidth := (3*channel_bit + 3) / 4
	for _, key := range hist_keys_sorted {
		count := histogram[key]
		_, err := fmt.Fprintf(file, "Color: 0x%0*X, Count: %d\n", hexWidth, key, count)
		if err != nil {
			return fmt.Errorf("error writing to histogram file: %v\n", err)
		}
	}
	return nil
}

func check_settings() error {
	if channel_bit < channel_bit_threshold[0] || channel_bit > channel_bit_threshold[1] {
		return fmt.Errorf("channel_bit must be between %d and %d", channel_bit_threshold[0], channel_bit_threshold[1])
	}
	return nil
}

func main() {
	if err := check_settings(); err != nil {
		log.Fatalf("Invalid settings: %v\n", err)
	}
	histo_folder := fmt.Sprintf("%s_%d_bits_each", histo_folder, channel_bit)
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
		histo, hist_keys_sorted, err := image_detail(img, format)
		if err != nil {
			fmt.Printf("Error processing image %s: %v\n", name, err)
			continue
		}
		hist_save_name := filepath.Join(histo_folder, fmt.Sprintf("%s_%d_bits_each.txt", name, channel_bit))
		err_save_hist := save_histogram(histo, hist_keys_sorted, hist_save_name)
		if err_save_hist != nil {
			fmt.Printf("Error saving histogram for image %s: %v\n", name, err_save_hist)
		}
		fmt.Printf("%s: %s\n", name, format)
		entry_count++
	}
	fmt.Printf("Number of images in target folder: %d\n", entry_count)
}
