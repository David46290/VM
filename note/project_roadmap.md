# 專案時間線與階段規劃

## 總覽

原本的五步計畫（掃描 → 特徵分析 → 收集結果 → 定區分標準 → 分類）方向正確，修正版把「特徵」和「分群」之間加了一層快取，並在真正搬檔案前加了 dry-run 關卡：

```
掃描資料夾
  → 解碼 → (縮圖) → 算 histogram → 正規化成特徵向量
  → 特徵集合存檔 (JSON/CSV)          ← 快取層，讓調參可以重跑
  → 算兩兩距離 → 分群               ← 便宜，可重跑很多次
  → 印出分群報表 (dry-run，不搬檔)
  → 人工抽查確認
  → 才真正搬移/複製檔案 (並寫 log)
```

為什麼加快取層：第 4 步「定區分標準」注定要反覆試（換距離公式、調門檻、改 bin 數）。每次嘗試都重新解碼幾千張圖會調到崩潰。Python 的習慣要搬過來：特徵先存成檔案，之後只在特徵上做實驗。

## 目前所在階段

**Phase 2 判讀已完成（`channelBitDepth = 2` 的人工對照）**；下一步是依判讀結果決定要不要處理背景／調 bit，再進 Phase 3。詳見 `histogram_and_clustering.md` 第 2_1 節。

## 階段表

### Phase 0 — 掃描與解碼（完成）

- [x] `os.ReadDir` 掃描 `images/`、跳過子資料夾
- [x] `validateFile`：開檔、`image.Decode`、`defer file.Close()` 包成單一函式
- [x] 用 type switch 探索過 `*image.YCbCr` / `*image.RGBA` 等底層型別（學習目的已達成）
- [x] 占位字串已被真正的 histogram 管線取代

註：小相簿用 `os.ReadDir` 只掃一層就夠；大相簿若有子資料夾，Phase 5 再換 `filepath.WalkDir`。

### Phase 1 — Histogram 特徵（完成）

目標：把每張圖變成正規化特徵，並以 struct 綁定路徑。

待辦：

- [x] 3D 聯合 histogram + 每通道 `channelBitDepth` 量化；正規化為 `map[uint32]float64` 標準表示
- [x] 不同 bit 設定分目錄存放；bit 數實驗 2–6（見 `histogram_and_clustering.md` 1_1）
- [x] `histKeyDecode`、紅／藍簽名格驗證（見 1_2）
- [x] 熱迴圈效能：字串 map → 固定變數（約快 10 秒）
- [x] `ImageHist` struct：`Path` / `Width` / `Height` / `Histogram`（**不**握 `image.Image`，避免把解碼圖留在記憶體）
- [x] `generateHistogram` 回傳 `[]ImageHist`；`saveHistogram`／`saveHistogramFile` 分開，存檔可選
- [x] 命名改為 Go MixedCaps（`validateFile`、`channelBitDepth` 等）

驗收：

- [x] 複製圖 histogram 一致；count 總和與正規化總和檢查；紅藍簽名 bin 符合預期

### Phase 2 — 距離與探索報告（判讀完成；對策未定案）

目標：當探測器，回答三個問題（結論見 `histogram_and_clustering.md` 2_1）。

待辦：

- [x] L1 距離：`correlateHistL1`，缺 key 當 0、無序配對不重複、自比跳過
- [x] 依分數排序輸出；印最近／最遠各 N 對（`l1ScoreShowAmount`）
- [x] 人工對照（`channelBitDepth = 2`）：紀錄於 `output/debug/l1_coorelate/bitDepth2_l1_coorelation_inspect.txt`
- [ ] 用 `channelBitDepth = 4` 重跑最近／最遠 10 對，對照「不同模型同舞台」是否仍擠進前段
- [ ] 依對照結果決定下一步對策（背景處理／bit 定案／是否換特徵）

### Phase 3 — 特徵快取與分群調參

- [ ] 特徵存檔（先求有：JSON/CSV），分群實驗只讀特徵檔
- [ ] 依 Phase 2 對策調參：背景、bin、門檻、是否換 HSV 等
- [ ] 產出分群報表，人工抽查代表照片

### Phase 4 — Goroutine 加速（見 `goroutine_concurrency.md`）

前置：Phase 1–3 單線程正確，且量過基準時間。

- [ ] 特徵萃取改 worker pool
- [ ] 與單線程結果一致（排序後比對）
- [ ] 再量時間

### Phase 5 — 上大相簿

- [ ] 必要時改 `filepath.WalkDir`
- [ ] dry-run 報表 → 人工抽查 → 搬移／複製並寫 log

## 學習節奏備忘

- 同一時間只跟一個敵人打架：演算法 bug 和併發 bug 不要同時出現。
- 每個 Phase 結束，把踩到的 Go 坑補進 `go_pitfalls_checklist.md` 或 `go_notes.md`。
- 卡住時先定位自己卡在「哪個設計決定」，再查筆記或提問。
