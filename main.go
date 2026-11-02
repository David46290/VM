package main

import (
	"cmp"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const epsilon = 1e-9

var (
	targetFolder             = "images"
	supportedImgType         = [2]string{"jpeg", "png"}
	channelBitDepth          = 4             // bit depth for each channel (default 4 for histogram generation. Lower values for quick debugging)
	channelBitDepthThreshold = [2]int{1, 10} //max 10 to prevent overflow of uint32 in histogram key
	imageDefaultBitDepth     = 16
	// bit position for each channel in the histogram key, 2: leftmost, 0: rightmost
	channelBitPlaceR  = 2
	channelBitPlaceG  = 1
	channelBitPlaceB  = 0
	folderSuffix      = "bits_each_rgb_norm"
	histoFolder       = filepath.Join("output/debug/histogram", fmt.Sprintf("%d_%s", channelBitDepth, folderSuffix))
	histoSaveSuffix   = "bits_each_rgb_norm.txt"
	l1Folder          = "output/debug/l1_coorelate"
	l1SaveSuffix      = "l1_coorelation.txt"
	l1ScoreShowAmount = 10
)

type ImageHist struct {
	Path      string
	Width     int
	Height    int
	Histogram map[uint32]float64
}

func checkSettings() error {
	if channelBitDepth < channelBitDepthThreshold[0] || channelBitDepth > channelBitDepthThreshold[1] {
		return fmt.Errorf("channelBitDepth must be between %d and %d", channelBitDepthThreshold[0], channelBitDepthThreshold[1])
	}
	return nil
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

func sortHist(m map[uint32]float64) ([]uint32, error) {
	keys := make([]uint32, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, nil
}

func sortL1(m map[string]float64) ([]string, error) {
	// slices.SortedFunc(iteratable_to_be_sorted, func_of_sorting)
	// func_of_sorting(a, b): return -1 a<b, 0 a=b, 1 a>b
	// SortedFunc() compares each pair in the iterable using func_of_sorting.
	// In this case func_of_sorting(anonymous, like lambda in Python) uses cmp.Compare to compare values (or keys if same values are met).
	// cmp.Or(cmp.Compare(m[a], m[b]), cmp.Compare(a, b)) is used to sort by value first, then by key if values are the same.
	keys := slices.SortedFunc(maps.Keys(m), func(a, b string) int {
		return cmp.Or(
			cmp.Compare(m[a], m[b]),
			cmp.Compare(a, b),
		)
	})
	return keys, nil
}

func histCountInspect(histogram map[uint32]int, bounds image.Rectangle) error {
	sum := 0
	for _, count := range histogram {
		sum += count
	}

	imgHeight := bounds.Max.Y - bounds.Min.Y
	imgWidth := bounds.Max.X - bounds.Min.X
	expectedCount := imgHeight * imgWidth
	if sum != expectedCount {
		return fmt.Errorf("histogram count mismatch: expected %d, got %d", expectedCount, sum)
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

func generateHistogram(targetDir string) ([]ImageHist, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("can't read target folder: %v\n", err)
	}

	albumHisto := []ImageHist{}
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
		albumHisto = append(albumHisto, imageHist)
	}

	elapsed := time.Since(start)
	fmt.Printf("Number of images in target folder: %d\n", len(albumHisto))
	fmt.Printf("Histogram generation time: %v\n", elapsed)
	return albumHisto, nil
}

func saveHistogramFile(imageHist ImageHist) error {
	base := filepath.Base(imageHist.Path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	histSaveName := filepath.Join(histoFolder, fmt.Sprintf("%s_%d_%s", name, channelBitDepth, histoSaveSuffix))
	file, err := os.Create(histSaveName)
	if err != nil {
		return fmt.Errorf("error creating histogram file: %v\n", err)
	}
	defer file.Close()
	histogram := imageHist.Histogram
	sortedKeys, err := sortHist(histogram)
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
func saveHistogram(albumHisto []ImageHist) error {
	if err := os.MkdirAll(histoFolder, 0o755); err != nil {
		return err
	}
	for _, imageHist := range albumHisto {
		if err := saveHistogramFile(imageHist); err != nil {
			return fmt.Errorf("error saving histogram file: %v\n", err)
		}
	}
	return nil
}

func correlateHistL1(albumHisto []ImageHist) (map[string]float64, error) {
	checkedName := make(map[string]bool)
	l1records := make(map[string]float64)
	start := time.Now()
	for _, imageHistA := range albumHisto {
		// Mark A as checked first because A doesn't check with itself.
		checkedName[imageHistA.Path] = true
		for _, imageHistB := range albumHisto {
			if _, isExist := checkedName[imageHistB.Path]; isExist {
				// If B in checkedName, that means it was A before, and had already been checked w/ current A.
				// So it should not goes through the same process again.
				// _: key's value, isExist: "comma-ok"
				// In GoLang, if two results need to be returned in process ":= map[key]",
				// the 2nd returned value will be a state flag telling "if the key exists in the map".
				// It also works on type assertion & channel opperation.
				continue
			}
			l1Distance := 0.0
			for key, countA := range imageHistA.Histogram {
				// Get distance on keys that exist in both histograms
				// Golang returns 0 from map[key] automatically if the key is not in the map
				l1Distance += math.Abs(countA - imageHistB.Histogram[key])
			}
			for key, countB := range imageHistB.Histogram {
				// Get distance on keys that exist in imageHistB but not in imageHistA
				if _, isExist := imageHistA.Histogram[key]; !isExist {
					l1Distance += math.Abs(countB)
				}
			}
			baseA := filepath.Base(imageHistA.Path)
			nameA := strings.TrimSuffix(baseA, filepath.Ext(baseA))
			baseB := filepath.Base(imageHistB.Path)
			nameB := strings.TrimSuffix(baseB, filepath.Ext(baseB))
			recordKey := fmt.Sprintf("%s_vs_%s", nameA, nameB)
			l1records[recordKey] = l1Distance
		}
	}
	elapsed := time.Since(start)
	fmt.Printf("L1 correlation time: %v\n", elapsed)
	return l1records, nil
}

func saveL1Records(l1records map[string]float64) error {
	if err := os.MkdirAll(l1Folder, 0o755); err != nil {
		return err
	}
	recordSaveName := filepath.Join(l1Folder, fmt.Sprintf("bitDepth%d_%s", channelBitDepth, l1SaveSuffix))
	file, err := os.Create(recordSaveName)
	if err != nil {
		return fmt.Errorf("error creating l1 record file: %v\n", err)
	}
	defer file.Close()
	sortedKeys, err := sortL1(l1records)
	if err != nil {
		return fmt.Errorf("error sorting l1 keys: %v\n", err)
	}
	for _, key := range sortedKeys {
		_, err := fmt.Fprintf(file, "%s: %f\n", key, l1records[key])
		if err != nil {
			return fmt.Errorf("error writing to l1 record file: %v\n", err)
		}
	}

	// Print top 10 lowest l1 scores
	lowestIndexes := min(l1ScoreShowAmount, len(sortedKeys))
	for i, key := range sortedKeys[:lowestIndexes] {
		_, err := fmt.Printf("Lowest %d: %s: %f\n", i+1, key, l1records[key])
		if err != nil {
			return fmt.Errorf("error printing top 10 lowest l1 scores: %v\n", err)
		}
	}

	// Print top 10 highest l1 scores
	highestIndexes := max(0, len(sortedKeys)-l1ScoreShowAmount)
	for i, key := range sortedKeys[highestIndexes:] {
		_, err := fmt.Printf("Highest %d: %s: %f\n", i+1, key, l1records[key])
		if err != nil {
			return fmt.Errorf("error printing top 10 highest l1 scores: %v\n", err)
		}
	}
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

	albumHisto, histoErr := generateHistogram(targetDir)
	if histoErr != nil {
		fmt.Printf("error generating histogram: %v\n\n", histoErr)
		os.Exit(1)
	}
	saveErr := saveHistogram(albumHisto)
	if saveErr != nil {
		fmt.Printf("error saving histogram: %v\n\n", saveErr)
		os.Exit(1)
	}
	l1records, l1Err := correlateHistL1(albumHisto)
	if l1Err != nil {
		fmt.Printf("error correlating histogram: %v\n\n", l1Err)
		os.Exit(1)
	}
	saveL1Err := saveL1Records(l1records)
	if saveL1Err != nil {
		fmt.Printf("error saving l1 records: %v\n\n", saveL1Err)
		os.Exit(1)
	}

}
