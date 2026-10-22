package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const epsilon = 1e-9

var (
	target_folder         = "images"
	supported_img_type    = [2]string{"jpeg", "png"}
	channel_bit           = 4             // bit depth for each channel (default 4 for histogram generation)
	channel_bit_threshold = [2]int{1, 10} //max 10 to prevent overflow of uint32 in histogram key
	default_channel_bit   = 16
	// bit position for each channel in the histogram key, 2: leftmost, 0: rightmost
	channel_bit_place_r = 2
	channel_bit_place_g = 1
	channel_bit_place_b = 0
	folder_suffix       = "bits_each_rgb_norm"
	histo_folder        = filepath.Join("output/debug/histogram", fmt.Sprintf("%d_%s", channel_bit, folder_suffix))
	histo_save_suffix   = "bits_each_rgb_norm.txt"
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

func map_sort_keys(m map[uint32]float64) ([]uint32, error) {
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

func hist_normalize_check(histogram map[uint32]float64) error {
	sum := 0.0
	for _, magnitude := range histogram {
		sum += magnitude
	}
	if math.Abs(sum-1.0) > epsilon {
		return fmt.Errorf("histogram is not normalized: sum = %f", sum)
	}
	return nil
}

func hist_normalize(histogram map[uint32]int) map[uint32]float64 {
	count_total := 0
	for _, c := range histogram {
		count_total += c
	}
	normalized := make(map[uint32]float64)
	for key, count := range histogram {
		normalized[key] = float64(count) / float64(count_total)
	}
	return normalized
}

func img_histogram(img image.Image) (map[uint32]float64, []uint32, error) {
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
			key := (r_shrink << (channel_bit_place_r * channel_bit)) | (g_shrink << (channel_bit_place_g * channel_bit)) | (b_shrink << (channel_bit_place_b * channel_bit))
			histogram[key]++

		}
	}
	if err := hist_count_inspect(histogram, bounds); err != nil {
		return nil, nil, err
	}
	histogram_norm := hist_normalize(histogram)
	if err := hist_normalize_check(histogram_norm); err != nil {
		return nil, nil, err
	}
	sorted_keys, err := map_sort_keys(histogram_norm)
	if err != nil {
		return nil, nil, err
	}
	return histogram_norm, sorted_keys, nil
}

func image_detail(img image.Image, format string) (map[uint32]float64, []uint32, error) {
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

func hist_key_decode(key uint32) (uint32, uint32, uint32) {
	// Decode the histogram key back into RGB values
	// Push specific bits to the right (amount of channel_bit_place)
	// Then mask with (1 << channel_bit) - 1 to get the original value
	r := (key >> (channel_bit_place_r * channel_bit)) & ((1 << channel_bit) - 1)
	g := (key >> (channel_bit_place_g * channel_bit)) & ((1 << channel_bit) - 1)
	b := (key >> (channel_bit_place_b * channel_bit)) & ((1 << channel_bit) - 1)
	return r, g, b
}

func bright_percentage(value uint32) float64 {
	// Normalize the value to the range [0, 1] based on channel_bit
	// channel depth = channel_bit, so max value (brightest) = 2^channel_bit - 1
	max_value := (1 << channel_bit) - 1
	return float64(value) / float64(max_value)
}

func save_histogram(histogram map[uint32]float64, hist_keys_sorted []uint32, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating histogram file: %v\n", err)
	}
	defer file.Close()

	for _, key := range hist_keys_sorted {
		count := histogram[key]
		r, g, b := hist_key_decode(key)
		r_percentage := bright_percentage(r)
		g_percentage := bright_percentage(g)
		b_percentage := bright_percentage(b)
		_, err := fmt.Fprintf(file, "RGB: (%.3f, %.3f, %.3f), contribution: %.5f\n", r_percentage, g_percentage, b_percentage, count)
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

func generate_histo(target_dir string) error {
	if err := os.MkdirAll(histo_folder, 0o755); err != nil {
		return fmt.Errorf("error creating histogram folder: %v\n", err)
	}
	entries, err := os.ReadDir(target_dir)
	if err != nil {
		return fmt.Errorf("can't read target folder: %v\n", err)
	}

	entry_count := 0
	start := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(target_dir, name)
		img, format, err := validate_file(path)
		if err != nil {
			fmt.Printf("file %s is not a valid image.\n", name)
			continue
		}
		histo, hist_keys_sorted, err := image_detail(img, format)
		if err != nil {
			fmt.Printf("error processing image %s: %v\n", name, err)
			continue
		}
		hist_save_name := filepath.Join(histo_folder, fmt.Sprintf("%s_%d_%s", name, channel_bit, histo_save_suffix))
		err_save_hist := save_histogram(histo, hist_keys_sorted, hist_save_name)
		if err_save_hist != nil {
			fmt.Printf("error saving histogram for image %s: %v\n", name, err_save_hist)
		}
		// fmt.Printf("%s: %s\n", name, format)
		entry_count++
	}
	elapsed := time.Since(start)
	fmt.Printf("Number of images in target folder: %d\n", entry_count)
	fmt.Printf("Elapsed time: %v\n", elapsed)
	return nil
}

func main() {
	if err := check_settings(); err != nil {
		fmt.Printf("Invalid settings: %v\n\n", err)
		os.Exit(1)
	}

	current_dir, _ := os.Getwd()
	target_dir, _ := filepath.Abs(target_folder)
	fmt.Printf("Current working directory: %s\n", current_dir)
	fmt.Printf("Target folder: %s\n", target_dir)
	histo_err := generate_histo(target_dir)
	if histo_err != nil {
		fmt.Printf("error generating histogram: %v\n\n", histo_err)
		os.Exit(1)
	}
}
