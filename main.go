package main

import (
	"cmp"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

const epsilon = 1e-9

var (
	targetFolder             = "imagesQuickDebug"
	supportedImgType         = [2]string{"jpeg", "png"}
	channelBitDepth          = 4             // bit depth for each channel (default 4 for histogram generation. Lower values for quick debugging)
	channelBitDepthThreshold = [2]int{1, 10} //max 10 to prevent overflow of uint32 in histogram key
	imageDefaultBitDepth     = 16
	imageProcessBitDepth     = 8
	// bit position for each channel in the histogram key, 2: leftmost, 0: rightmost
	channelBitPlaceR    = 2
	channelBitPlaceG    = 1
	channelBitPlaceB    = 0
	folderSuffix        = "bits_each_rgb_norm"
	histoFolder         = filepath.Join("output/debug/histogram", fmt.Sprintf("%d_%s", channelBitDepth, folderSuffix))
	histoSaveSuffix     = "bits_each_rgb_norm.txt"
	l1Folder            = "output/debug/l1_coorelate"
	l1SaveSuffix        = "l1_coorelation.txt"
	l1ScoreShowAmount   = 10
	heatMapFolder       = "output/debug/heatmap"
	heatMapSaveSuffix   = "heatmap_from_smoothed.jpg"
	roiFolder           = "output/debug/roi"
	roiSaveSuffix       = "roi.jpg"
	roiContourThickness = 2
	smoothedFolder      = "output/debug/smoothed"
	smoothedSaveSuffix  = "smoothed.jpg"
	stageOptions        = map[string]int{"smoothed": 1, "heatmap": 2, "roi": 3, "hist": 4, "l1": 5}
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

	// EXIF Orientation Correction (for smartphone photos)
	img, err := imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", err
	}

	ext := strings.ToLower(filepath.Ext(path))
	format := strings.TrimPrefix(ext, ".")
	if format == "jpg" {
		format = "jpeg"
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

func gaussianSmoothing(img image.Image) (image.Image, error) {
	kernelSize := 3
	kernel := [3][3]int{
		{1, 2, 1},
		{2, 4, 2},
		{1, 2, 1},
	}

	bounds := img.Bounds()
	// Zero-padding
	// center-moving window
	if kernelSize%2 == 0 {
		return nil, fmt.Errorf("kernel_size must be odd")
	}
	kernelWingSize := kernelSize / 2
	convResult := image.NewGray(bounds)
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			// Total counting index: (i-kernelWingSize : i+ kernelWingSize, j-kernelWingSize : j+ kernelWingSize)
			sumWholeKernel := 0
			for kx := 0; kx < kernelSize; kx++ {
				for ky := 0; ky < kernelSize; ky++ {
					x := i + kx - kernelWingSize
					y := j + ky - kernelWingSize
					if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
						continue
					}
					r, g, b, _ := img.At(x, y).RGBA()
					gray := int(((r + g + b) / 3) >> (imageDefaultBitDepth - imageProcessBitDepth)) // 0~255
					sumWholeKernel += kernel[kx][ky] * gray
				}
			}
			convResult.SetGray(i, j, color.Gray{Y: uint8(sumWholeKernel / 16)})
		}
	}
	return convResult, nil
}

func heatMapGeneration(img image.Image) (image.Image, error) {
	grayImg, ok := img.(*image.Gray)
	if !ok {
		return nil, fmt.Errorf("heatMapGeneration expects *image.Gray after smoothing")
	}
	kernelSize := 3
	kernelX := [3][3]int{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	}
	kernelY := [3][3]int{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	}
	bounds := grayImg.Bounds()
	// Zero-padding
	// center-moving window
	if kernelSize%2 == 0 {
		return nil, fmt.Errorf("kernel_size must be odd")
	}
	kernelWingSize := kernelSize / 2
	convResult := image.NewGray(bounds)
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			// Total counting index: (i-kernelWingSize : i+ kernelWingSize, j-kernelWingSize : j+ kernelWingSize)
			sumWholeKernelX := 0
			sumWholeKernelY := 0
			for kx := 0; kx < kernelSize; kx++ {
				for ky := 0; ky < kernelSize; ky++ {
					x := i + kx - kernelWingSize
					y := j + ky - kernelWingSize
					if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
						continue
					}
					gray := int(grayImg.GrayAt(x, y).Y)
					sumWholeKernelX += kernelX[kx][ky] * gray
					sumWholeKernelY += kernelY[kx][ky] * gray
				}
			}
			convResult.SetGray(i, j, color.Gray{Y: uint8(math.Sqrt(float64(sumWholeKernelX*sumWholeKernelX + sumWholeKernelY*sumWholeKernelY)))})
		}
	}
	return convResult, nil
}

func roiDetect(imgHeatMap image.Image) ([]int, error) {
	//return 4 integers: minX, minY, maxX, maxY
	bounds := imgHeatMap.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y
	cropSegWidth := int(float64(width)*0.6) / 2
	cropSegHeight := int(float64(height)*0.6) / 2
	minX := bounds.Min.X + cropSegWidth
	minY := bounds.Min.Y + cropSegHeight
	maxX := bounds.Max.X - cropSegWidth
	maxY := bounds.Max.Y - cropSegHeight
	return []int{minX, minY, maxX, maxY}, nil
}

func drawRoi(img image.Image, roi []int) (image.Image, error) {
	//copy a drawable image instance from img
	target := image.NewRGBA(img.Bounds())
	fmt.Printf("Image size: %d x %d\n", img.Bounds().Dx(), img.Bounds().Dy())
	draw.Draw(target, target.Bounds(), img, img.Bounds().Min, draw.Src)
	// draw ROI contour
	x0 := roi[0]
	x1 := roi[2]
	y0 := roi[1]
	y1 := roi[3]
	whiteBackground := &image.Uniform{C: color.White}
	draw.Draw(target, image.Rect(x0, y0, x1, y0+roiContourThickness), whiteBackground, image.Point{}, draw.Src) //upside
	draw.Draw(target, image.Rect(x0, y0, x0+roiContourThickness, y1), whiteBackground, image.Point{}, draw.Src) //leftside
	draw.Draw(target, image.Rect(x1-roiContourThickness, y0, x1, y1), whiteBackground, image.Point{}, draw.Src) //rightside
	draw.Draw(target, image.Rect(x0, y1-roiContourThickness, x1, y1), whiteBackground, image.Point{}, draw.Src) //downside
	return target, nil
}

func saveRoiMarking(img image.Image, folderDir string, name string, imgType string) error {
	saveName := filepath.Join(folderDir, fmt.Sprintf("%s_%s_%s", name, imgType, roiSaveSuffix))
	file, err := os.Create(saveName)
	if err != nil {
		return fmt.Errorf("error creating roi file: %v\n", err)
	}
	defer file.Close()
	err = jpeg.Encode(file, img, nil)
	if err != nil {
		return fmt.Errorf("error encoding roi marked image: %v\n", err)
	}
	return nil
}

func drawAndSaveRoi(imgOriginal image.Image, imgHeatMap image.Image, roi []int, name string) error {
	// draw ROI contour on both original image & heat map
	imgRoiFolder := filepath.Join(roiFolder, name)
	if err := os.MkdirAll(imgRoiFolder, 0o755); err != nil {
		return err
	}
	markedOriginal, err := drawRoi(imgOriginal, roi)
	if err != nil {
		return fmt.Errorf("error drawing roi: %v\n", err)
	}
	err = saveRoiMarking(markedOriginal, imgRoiFolder, name, "og")
	if err != nil {
		return fmt.Errorf("error saving roi marked image: %v\n", err)
	}
	markedHeatMap, err := drawRoi(imgHeatMap, roi)
	if err != nil {
		return fmt.Errorf("error drawing roi: %v\n", err)
	}
	err = saveRoiMarking(markedHeatMap, imgRoiFolder, name, "heatmap")
	if err != nil {
		return fmt.Errorf("error saving roi marked image: %v\n", err)
	}
	return nil
}

func imageProcess(targetDir string, stopStageIdx int) ([]ImageHist, int, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, 0, fmt.Errorf("can't read target folder: %v\n", err)
	}
	albumHisto := []ImageHist{}
	start := time.Now()
	stageIdx := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		nameWithExT := entry.Name()
		base := filepath.Base(nameWithExT)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		path := filepath.Join(targetDir, nameWithExT)
		img, format, err := validateFile(path)
		if err != nil {
			fmt.Printf("file %s is not a valid image.\n", nameWithExT)
			continue
		}
		imgSmoothed, err := gaussianSmoothing(img)
		if err != nil {
			fmt.Printf("error smoothing image: %v\n", err)
			continue
		}
		saveGrayMap(imgSmoothed, name, smoothedFolder, smoothedSaveSuffix)
		stageIdx = stageOptions["smoothed"]
		if stageIdx >= stopStageIdx {
			continue
		}

		imgHeatMap, err := heatMapGeneration(imgSmoothed)
		if err != nil {
			fmt.Printf("error generating heat map: %v\n", err)
			continue
		}
		saveGrayMap(imgHeatMap, name, heatMapFolder, heatMapSaveSuffix)
		stageIdx = stageOptions["heatmap"]
		if stageIdx >= stopStageIdx {
			continue
		}

		roi, err := roiDetect(imgHeatMap)
		if err != nil {
			fmt.Printf("error detecting ROI: %v\n", err)
			continue
		}
		sub, ok := img.(interface {
			SubImage(r image.Rectangle) image.Image
		})
		if !ok {
			fmt.Printf("image does not support SubImage: %v\n", err)
			continue
		}
		err = drawAndSaveRoi(img, imgHeatMap, roi, name)
		if err != nil {
			fmt.Printf("error drawing and saving roi: %v\n", err)
			continue
		}
		stageIdx = stageOptions["roi"]
		if stageIdx >= stopStageIdx {
			continue
		}

		imgCropped := sub.SubImage(image.Rect(roi[0], roi[1], roi[2], roi[3]))
		imageHist, err := histogramExtract(path, imgCropped, format)
		if err != nil {
			fmt.Printf("error processing image %s: %v\n", name, err)
			continue
		}
		albumHisto = append(albumHisto, imageHist)
	}

	elapsed := time.Since(start)
	fmt.Printf("Number of images in target folder: %d\n", len(albumHisto))
	fmt.Printf("Image processing time: %v\n", elapsed)
	return albumHisto, stageIdx, nil
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
			recordKey := fmt.Sprintf("%s vs %s", nameA, nameB)
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

func saveGrayMap(img image.Image, name string, categoryFolder string, categorySuffix string) error {
	if err := os.MkdirAll(categoryFolder, 0o755); err != nil {
		return err
	}
	saveName := filepath.Join(categoryFolder, fmt.Sprintf("%s_%s", name, categorySuffix))
	file, err := os.Create(saveName)
	if err != nil {
		return fmt.Errorf("error creating %s file: %v\n", categorySuffix, err)
	}
	defer file.Close()
	err = jpeg.Encode(file, img, nil)
	if err != nil {
		return fmt.Errorf("error encoding %s: %v\n", categorySuffix, err)
	}
	fmt.Printf("%s saved to %s\n", categorySuffix, saveName)
	return nil
}

func main() {
	stopStageString := flag.String("stopStage", "heatmap", "stop at stage: smoothed|heatmap|roi|hist|l1")
	flag.Parse()
	stopStageIdx := stageOptions[*stopStageString]

	if stopStageIdx == 0 {
		fmt.Printf("Wrong stage name: %s\nShutting down...", *stopStageString)
		os.Exit(1)
	}

	if err := checkSettings(); err != nil {
		fmt.Printf("Invalid settings: %v\n\n", err)
		os.Exit(1)
	}
	currentDir, _ := os.Getwd()
	targetDir, _ := filepath.Abs(targetFolder)
	fmt.Printf("Current working directory: %s\n", currentDir)
	fmt.Printf("Target folder: %s\n", targetDir)
	fmt.Printf("Channel bits: %d\n", channelBitDepth)

	albumHisto, stageIdx, histoErr := imageProcess(targetDir, stopStageIdx)
	if histoErr != nil {
		fmt.Printf("error generating histogram: %v\n\n", histoErr)
		os.Exit(1)
	}
	if stageIdx >= stopStageIdx {
		fmt.Printf("Stop at stage: %s\n", *stopStageString)
		os.Exit(0)
	}

	saveErr := saveHistogram(albumHisto)
	if saveErr != nil {
		fmt.Printf("error saving histogram: %v\n\n", saveErr)
		os.Exit(1)
	}
	stageIdx = stageOptions["hist"]
	if stageIdx >= stopStageIdx {
		fmt.Printf("Stop at stage: %s\n", *stopStageString)
		os.Exit(0)
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
