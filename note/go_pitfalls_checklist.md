# Go 速查表：本專案會撞到的坑

寫碼前掃一遍用的速查表。觀念的詳細版在 `go_notes.md`，這裡只留「會咬人的一句話版本」。

## `image` 套件相關

- [ ] `img.At(x, y).RGBA()` 回傳 **16-bit**（0–65535）的 r, g, b, a，不是 0–255。算 bin 前要先映射。
- [ ] 圖片範圍用 `Bounds()`：起點是 `Bounds().Min`，**不保證是 (0,0)**；寬高是 `Bounds().Dx()`、`Bounds().Dy()`。迴圈要用 Min→Max，不要用 0→寬。
- [ ] 算 histogram **不需要知道底層型別**。`*image.YCbCr`、`*image.RGBA`、`*image.Gray` 是解碼器的儲存方式；`At().RGBA()` 已抽象成統一的顏色值。`image_type` 那個 type switch 是學習探索，做特徵時可以退休。
- [ ] `ColorModel()` 是拿來轉換顏色表示的，不是拿來數通道的；它不會出現在 debugger 的欄位展開裡。要看型別用 `fmt.Printf("%T\n", img)`。
- [ ] `image.Decode` 能認格式靠空白匯入註冊：`_ "image/jpeg"`、`_ "image/png"` 要留在 import 裡，拿掉就解不了對應格式。
- [ ] 逐像素 `At()` 走介面呼叫是慢的。小相簿驗證正確性可以接受；「慢到受不了」時先想縮圖，不要先想 goroutine。

## 效能與 map 相關

- [ ] **熱迴圈裡不要查 map，尤其是字串 key。** 每像素執行的程式碼，任何查表都會被乘上億次；實測把每像素 3 次字串 map 查表改成固定變數，整批快約 10 秒（實驗數據見 `histogram_and_clustering.md` 效能備忘）。迴圈不變量（位移量、遮罩等）在迴圈開始前算好。
- [ ] Go 查 map 不存在的 key 會**靜默回傳零值**，不報錯。用 map 存設定時，key 打錯字等於把值默默變成 0；固定少量的已知項目（例如 R/G/B 的位置）用常數或個別變數表達，不要用字串 map。
- [ ] 量測效能要記錄完整條件：資料量、參數設定、改前與改後的絕對時間。只有「快了 X 秒」沒有分母，之後無法跟其他優化比較。

## 檔案與流程相關

- [ ] `defer file.Close()` 的範圍是**所在的函式**：放在處理單張的子函式裡；放在迴圈所在的外層函式裡，會等函式結束才關 → 幾千張時檔案描述符爆掉。批次存檔應 `saveXxxFile` 一檔一函式 + `defer`（`saveHistogramFile` 已如此）。
- [ ] `os.ReadDir` 只掃一層、且回傳的 `DirEntry` 不含父路徑，要用 `filepath.Join(dir, entry.Name())` 拼完整路徑。要遞迴子資料夾改用 `filepath.WalkDir`（Phase 5）。
- [ ] 從完整路徑取無副檔名檔名：`base := filepath.Base(path)`，再 `strings.TrimSuffix(base, filepath.Ext(base))`。
- [ ] `filepath.Dir(完整檔案路徑)` 在「檔案就放在 `histoFolder` 底下」時，結果等於 `histoFolder`；外層已 `MkdirAll(histoFolder)` 就不必在內層再對 `Dir(檔名)` 建一次。
- [ ] 資料夾讀不到 → `main` 決定是否退出；單張失敗 → 印檔名後 `continue`。做事的函式回傳 `error`，不要自己 `log.Fatalf`。
- [ ] 長期存活的成果 struct（如 `ImageHist`）**不要**握 `image.Image`；解碼圖是工作材料，用完即丟。

## 錯誤處理相關

- [ ] Go 的慣例是回傳 `(結果, error)`，呼叫端立刻 `if err != nil` 決定處置，而不是回傳 bool 把原因吞掉。
- [ ] 空字串、nil、`0` 是失敗時的零值佔位；看到函式簽名有 `error`，先想「失敗時其他回傳值長什麼樣」。
- [ ] `panic` 留給程式沒救的狀況；「這張圖壞了」是可預期事件，用 error 回傳處理。

## 型別與變數相關

- [ ] 特徵資料用 struct 把**路徑和 histogram 綁在一起**（欄位自己命名），不要用兩個平行 slice 靠 index 對齊——併發版結果亂序時會對錯檔案。
- [ ] slice 是參照語意：把 histogram slice 傳來傳去時，想清楚「改到的是同一份還是複製」。函式要修改呼叫端的 slice 內容物可以，但要換整條（append 後長度變了）就得回傳新的。
- [ ] 浮點總和驗證不要用 `== 1.0`，要用「差距小於一個小 epsilon」來判斷。

## Debugger 速查（Delve）

- [ ] Variables 面板只展開欄位，不列方法。想看的呼叫先存成變數，在下一行下中斷點：
  `bounds := img.Bounds()` 然後在後面一行停下來看。
- [ ] Debug Console 直接呼叫方法可能被拒；先在程式碼裡把結果存下來再看。
- [ ] 型別問題先印 `%T`；API 問題先查 `go doc image.Image`、`go doc image.YCbCr`，不要盲翻標準庫原始碼。
- [ ] 組態問題先分清楚：`go run main.go` 是跑單檔，VS Code F5 是建置整個 package，後者需要 `go.mod`。

## Map／排序俚語（速記）

- [ ] `v, ok := m[k]`（comma-ok）：第二個值表示 key 是否存在；type assertion、channel 接收也有類似兩值形式。
- [ ] 只寫 `v := m[k]`：缺 key 得到零值，**分不出「真的是 0」和「不存在」**——算 L1 聯集時正好能用；存設定時則是地雷。
- [ ] 依 map 的 value 排序：`slices.SortedFunc(maps.Keys(m), func(a, b K) int { return cmp.Or(cmp.Compare(m[a], m[b]), cmp.Compare(a, b)) })`（Go 1.21+ 的 `cmp`／`maps`／`slices`）。

## 命名與格式（風格提醒）

- [ ] Go 用 **MixedCaps**：型別／匯出欄位 PascalCase（`ImageHist`）；套件內函式／變數小寫開頭 camelCase（`generateHistogram`）。**不要** snake_case，也**不要**把區域變數寫成大寫開頭（那表示匯出）。
- [ ] 縮寫維持全大寫風格：`ID`、`URL`；本專案用 `channelBitDepth`、`ImageHist` 這類完整或一致縮寫即可。
- [ ] 存檔後靠編輯器 `gofmt`；Tab 問題先確認沒誤觸 `Ctrl+M`（Toggle Tab Key Moves Focus）。
- [ ] 重構 commit 不要夾帶行為變更（例如順便改 `channelBitDepth`）；改名前後同設定產出應可逐字比對。
