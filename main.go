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
	targetFolder             = "images"
	supportedImgType         = [2]string{"jpeg", "png"}
	channelBitDepth          = 4             // bit depth for each channel (default 4 for histogram generation)
	channelBitDepthThreshold = [2]int{1, 10} //max 10 to prevent overflow of uint32 in histogram key
	imageDefaultBitDepth     = 16
	// bit position for each channel in the histogram key, 2: leftmost, 0: rightmost
	channelBitPlaceR = 2
	channelBitPlaceG = 1
	channelBitPlaceB = 0
	folderSuffix     = "bits_each_rgb_norm"
	histoFolder      = filepath.Join("output/debug/histogram", fmt.Sprintf("%d_%s", channelBitDepth, folderSuffix))
	histoSaveSuffix  = "bits_each_rgb_norm.txt"
)

type ImageHist struct {
	Path      string
	Width     int
	Height    int
	Histogram map[uint32]float64
}

func validateFile(path string) (image.Image, string, error) {
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

func mapSortKeys(m map[uint32]float64) ([]uint32, error) {
	keys := make([]uint32, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, nil
}

func histCountInspect(histogram map[uint32]int, bounds image.Rectangle) error {
	sum := 0
	for _, count := range histogram {
		sum += count
	}

	imgHeight := bounds.Max.Y - bounds.Min.Y
	imgWidth := bounds.Max.X - bounds.Min.X
	expected_count := imgHeight * imgWidth
	if sum != expected_count {
		return fmt.Errorf("histogram count mismatch: expected %d, got %d", expected_count, sum)
	}
	return nil
}

func histNormalizeCheck(histogram map[uint32]float64) error {
	sum := 0.0
	for _, magnitude := range histogram {
		sum += magnitude
	}
	if math.Abs(sum-1.0) > epsilon {
		return fmt.Errorf("histogram is not normalized: sum = %f", sum)
	}
	return nil
}

func histNormalize(histogram map[uint32]int) map[uint32]float64 {
	countTotal := 0
	for _, c := range histogram {
		countTotal += c
	}
	normalized := make(map[uint32]float64)
	for key, count := range histogram {
		normalized[key] = float64(count) / float64(countTotal)
	}
	return normalized
}

func getHist(img image.Image) (map[uint32]float64, error) {
	histogram := make(map[uint32]int)
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to 8-bit values
			rShrink := r >> (imageDefaultBitDepth - channelBitDepth)
			gShrink := g >> (imageDefaultBitDepth - channelBitDepth)
			bShrink := b >> (imageDefaultBitDepth - channelBitDepth)
			// Create a simple histogram key (you might want a more sophisticated approach)
			key := (rShrink << (channelBitPlaceR * channelBitDepth)) | (gShrink << (channelBitPlaceG * channelBitDepth)) | (bShrink << (channelBitPlaceB * channelBitDepth))
			histogram[key]++

		}
	}
	if err := histCountInspect(histogram, bounds); err != nil {
		return nil, err
	}
	histogramNorm := histNormalize(histogram)
	if err := histNormalizeCheck(histogramNorm); err != nil {
		return nil, err
	}

	return histogramNorm, nil
}

func histogramExtract(path string, img image.Image, format string) (ImageHist, error) {
	// get width, height, histogram
	if !slices.Contains(supportedImgType[:], format) {
		return ImageHist{}, fmt.Errorf("unsupported image format: %s\n", format)
	}
	histogram, err := getHist(img)
	if err != nil {
		return ImageHist{}, fmt.Errorf("error generating histogram: %v\n", err)
	}
	return ImageHist{
		Path:      path,
		Width:     img.Bounds().Dx(),
		Height:    img.Bounds().Dy(),
		Histogram: histogram,
	}, nil
}

func histKeyDecode(key uint32) (uint32, uint32, uint32) {
	// Decode the histogram key back into RGB values
	// Push specific bits to the right (amount of channelBitDepth_place)
	// Then mask with (1 << channelBitDepth) - 1 to get the original value
	r := (key >> (channelBitPlaceR * channelBitDepth)) & ((1 << channelBitDepth) - 1)
	g := (key >> (channelBitPlaceG * channelBitDepth)) & ((1 << channelBitDepth) - 1)
	b := (key >> (channelBitPlaceB * channelBitDepth)) & ((1 << channelBitDepth) - 1)
	return r, g, b
}

func brightPercentage(value uint32) float64 {
	// Normalize the value to the range [0, 1] based on channelBitDepth
	// channel depth = channelBitDepth, so max value (brightest) = 2^channelBitDepth - 1
	maximum := (1 << channelBitDepth) - 1
	return float64(value) / float64(maximum)
}

func saveHistogram(imageHist ImageHist, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating histogram file: %v\n", err)
	}
	defer file.Close()
	histogram := imageHist.Histogram
	sortedKeys, err := mapSortKeys(histogram)
	if err != nil {
		return fmt.Errorf("error sorting histogram keys: %v\n", err)
	}

	for _, key := range sortedKeys {
		count := histogram[key]
		r, g, b := histKeyDecode(key)
		rPercentage := brightPercentage(r)
		gPercentage := brightPercentage(g)
		bPercentage := brightPercentage(b)
		_, err := fmt.Fprintf(file, "RGB: (%.3f, %.3f, %.3f), contribution: %.5f\n", rPercentage, gPercentage, bPercentage, count)
		if err != nil {
			return fmt.Errorf("error writing to histogram file: %v\n", err)
		}
	}
	return nil
}

func checkSettings() error {
	if channelBitDepth < channelBitDepthThreshold[0] || channelBitDepth > channelBitDepthThreshold[1] {
		return fmt.Errorf("channelBitDepth must be between %d and %d", channelBitDepthThreshold[0], channelBitDepthThreshold[1])
	}
	return nil
}

func generateHistogram(targetDir string) error {
	if err := os.MkdirAll(histoFolder, 0o755); err != nil {
		return fmt.Errorf("error creating histogram folder: %v\n", err)
	}
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("can't read target folder: %v\n", err)
	}

	entryCount := 0
	start := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(targetDir, name)
		img, format, err := validateFile(path)
		if err != nil {
			fmt.Printf("file %s is not a valid image.\n", name)
			continue
		}
		imageHist, err := histogramExtract(path, img, format)
		if err != nil {
			fmt.Printf("error processing image %s: %v\n", name, err)
			continue
		}
		HistSaveName := filepath.Join(histoFolder, fmt.Sprintf("%s_%d_%s", name, channelBitDepth, histoSaveSuffix))
		errSaveHist := saveHistogram(imageHist, HistSaveName)
		if errSaveHist != nil {
			fmt.Printf("error saving histogram for image %s: %v\n", name, errSaveHist)
		}
		entryCount++
	}
	elapsed := time.Since(start)
	fmt.Printf("Number of images in target folder: %d\n", entryCount)
	fmt.Printf("Elapsed time: %v\n", elapsed)
	return nil
}

func main() {
	if err := checkSettings(); err != nil {
		fmt.Printf("Invalid settings: %v\n\n", err)
		os.Exit(1)
	}

	currentDir, _ := os.Getwd()
	targetDir, _ := filepath.Abs(targetFolder)
	fmt.Printf("Current working directory: %s\n", currentDir)
	fmt.Printf("Target folder: %s\n", targetDir)
	fmt.Printf("Channel bits: %d\n", channelBitDepth)
	histoErr := generateHistogram(targetDir)
	if histoErr != nil {
		fmt.Printf("error generating histogram: %v\n\n", histoErr)
		os.Exit(1)
	}
}
