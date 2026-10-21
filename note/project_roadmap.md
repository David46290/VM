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

## 階段表

### Phase 0 — 掃描與解碼（已大致完成）

- [x] `os.ReadDir` 掃描 `images/`、跳過子資料夾
- [x] `validate_file`：開檔、`image.Decode`、`defer file.Close()` 包成單一函式
- [x] 用 type switch 探索過 `*image.YCbCr` / `*image.RGBA` 等底層型別（學習目的已達成，做 histogram 時可以退休，見 `go_pitfalls_checklist.md`）
- [x] `image_detail` 的 `"histogram"` 占位字串已被真正的 histogram 函式取代（Phase 1）

註：小相簿用 `os.ReadDir` 只掃一層就夠；大相簿若有子資料夾，Phase 5 再換 `filepath.WalkDir` 遞迴走訪。

### Phase 1 — Histogram 特徵（目前所在階段，單線程）

目標：把每張圖變成一條正規化的 `[]float64` 特徵向量。

待辦：

- [x] 實作 histogram 函式（規格與設計抉擇見 `histogram_and_clustering.md`）：採 3D 聯合 histogram，每通道保留高 `channel_bit` 個 bit 打包成一個 key，存成 `map[uint32]int`（key → count），存檔前排序 key
- [x] 不同 bit 設定的產出分目錄存放、目錄與檔名都帶 bit 數，避免參數不同的特徵互相污染；舊版未量化產物已清除、產物目錄已加入 `.gitignore`
- [x] bit 數實驗：同一張圖掃過 `channel_bit` 2–6，記錄 distinct key 數（數據與 \(8^b\) 推導見 `histogram_and_clustering.md` 第 1_1 節）；暫定預設 `channel_bit = 4`，正式定案等 Phase 2
- [ ] 特徵 struct 從第一天就把路徑和特徵綁在一起（之後 goroutine 結果是亂序的，靠這個對回檔案）
- [ ] 正規化：目前存的是原始 count，算距離前要除以總像素數（留到 Phase 2 一起做）

驗收標準（三個自我測試全過才算完成）：

- [x] 同一張圖複製改名，兩份 histogram 完全相同（排序存檔後逐行比對一致）
- [x] 所有 count 總和 = 寬 × 高（由 `hist_count_inspect` 在程式內自動驗證；正規化版本「總和 ≈ 1.0」尚未做，併入上方正規化待辦）
- [ ] 明顯偏紅 vs 明顯偏藍的圖，高峰落在預期的 bin 附近

這一階段**不要**碰 goroutine、不要碰分群演算法。

### Phase 2 — 距離與探索報告

目標：先當探測器，不急著分類。回答三個問題：

1. 距離最近的照片對，看起來真的像嗎？（同色調？同背景？同一個模型？）
2. 距離最遠的照片對，差異來自模型本體，還是背景／光線？
3. 「不該像卻很近」的配對，兇手是什麼？（高機率是背景或大面積底色）

待辦：

- [ ] 距離函式（第一版用最簡單的逐 bin 絕對差總和即可）
- [ ] 小相簿兩兩算距離，印出「最近 10 對」與「最遠 10 對」檔名
- [ ] 人工對照照片判讀，結論寫回 `histogram_and_clustering.md`

### Phase 3 — 特徵快取與分群調參

- [ ] 特徵存檔（先求有：JSON/CSV 即可），分群實驗只讀特徵檔
- [ ] 依 Phase 2 的判讀決定對策：是否換 HSV、是否處理背景、bin 數與門檻怎麼定
- [ ] 產出分群報表，人工抽查每一群的代表照片

### Phase 4 — Goroutine 加速（見 `goroutine_concurrency.md`）

前置條件：Phase 1–3 單線程版本已正確，且量過基準時間。

- [ ] 特徵萃取改 worker pool
- [ ] 驗證平行版與單線程版結果一致（排序後比對）
- [ ] 再量一次時間，確認真的有變快、快多少

### Phase 5 — 上大相簿

- [ ] 掃描改 `filepath.WalkDir`（若有子資料夾）
- [ ] 先對 `model_kits` 跑 dry-run 報表，人工抽查
- [ ] 執行搬移/複製時寫 log（原路徑 → 新路徑），方便還原；注意同名檔案衝突的處理

## 學習節奏備忘

- 同一時間只跟一個敵人打架：演算法 bug 和併發 bug 不要同時出現。
- 每個 Phase 結束，把踩到的 Go 坑補進 `go_pitfalls_checklist.md` 或 `go_notes.md`。
- 卡住時先定位自己卡在「哪個設計決定」，再查筆記或提問，不要直接找完整寫法。
