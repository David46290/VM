# Goroutine 併發筆記

## 哪個階段適合平行

| 階段 | 適合程度 | 原因 |
|---|---|---|
| 特徵萃取（解碼＋算 histogram） | ★★★ 非常適合 | 每張圖完全獨立、互不等待，是教科書級的平行場景（embarrassingly parallel） |
| 兩兩距離／分群 | ★★ 可以但複雜 | 是全域運算：3000 張圖約 450 萬對比較，不像萃取那麼乾淨，先單線程跑通再說 |
| 搬移檔案 | ★ 不要 | 瓶頸在磁碟 I/O，平行好處少，還多出檔名衝突與錯誤回復的麻煩；依序搬＋寫 log 最安全 |

## 頭號禁止事項：一張圖開一個 goroutine

幾千張圖就開幾千個 goroutine 同時解碼，記憶體會直接爆：一張解碼後的圖片是幾十 MB 等級，同時活著幾百張就吃光了。也會同時開太多檔案。

正確的概念是 **worker pool（固定數量工人）**，對應 Python `ThreadPoolExecutor(max_workers=...)` 的想法，但 Go 要自己手刻：

```
main ──送路徑──▶ jobs channel ──▶ worker 1 ─┐
                                     worker 2 ─┼──▶ results channel ──▶ 收集者
                                     worker N ─┘
```

要自己想清楚的零件（這是本主題的功課，先懂概念再寫）：

- `jobs` channel：主程式把檔案路徑丟進去，worker 領工作。
- 固定 worker 數量：用 `runtime.NumCPU()` 當起點，不要用檔案數量。
- `results` channel：worker 把「路徑＋特徵」一起送回來。
- `sync.WaitGroup`：等所有 worker 收工；誰負責 `close(results)`、何時 close，是最容易 deadlock 的地方。
- worker 內部就是呼叫既有的單線程函式（解碼＋histogram），所以單線程版先寫好，平行版只是包一層調度。

## 併發版的兩個先天性質

1. **結果順序是亂的**。誰先算完誰先回來，所以特徵 struct 從 Phase 1 就要把檔案路徑和 histogram 綁在一起帶著走，不能靠順序對應檔名。
2. **錯誤也要走 channel 回來**。某張圖解碼失敗，worker 不能自己 `log.Fatal` 把全場帶走；要把「哪個檔案、什麼錯誤」跟結果一起回報，由收集者決定跳過或記錄。

## 採用流程（順序不准跳）

1. 單線程版完成且正確（過 Phase 1 驗收），量一次總時間當基準。
2. 寫 worker pool 版，只改調度，不改 histogram 邏輯。
3. 驗證兩版結果一致：平行版結果先照路徑排序，再和單線程版（或特徵快取檔）逐筆比對。
4. 再量一次時間，確認真的有變快、快多少；沒變快也要能解釋為什麼（例如瓶頸其實在磁碟讀取）。

同一時間只跟一個敵人打架：演算法還沒對的時候不要加併發，否則出錯分不清是分群邏輯錯還是 race／deadlock。

## 常見新手陷阱清單

- [ ] goroutine 數量失控（見上，禁止事項）
- [ ] channel 沒人 close → 收集者永遠等 → deadlock
- [ ] 多個 goroutine 同時寫同一個 slice／map → data race；結果一律走 channel 回主程式再彙整。驗證工具：`go run -race`
- [ ] 在 `main` 的迴圈裡 `defer file.Close()`：defer 的是 main，要等程式結束才關檔。開關檔要包在每張圖的處理函式裡（目前 `validate_file` 的切法就是對的）
- [ ] 用全域變數累積結果又不加同步——先用 channel 模式，`sync.Mutex` 是之後才需要認識的東西
