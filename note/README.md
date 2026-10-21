# 專案筆記索引

## 專案目標

用 Go 寫一支以顏色 histogram 為特徵的圖片分類工具。

- 練習場：專案內的 `images/` 小相簿，先在這裡把流程跑通、把參數調對。
- 最終目標：`C:\Users\Yooooooo\Pictures\model_kits` 裡幾千張模型照片的分類。
- 學習方式：程式碼由本人手刻，AI 只做引導、規格確認與 code review，不直接代寫完整實作。

目前決定的路線是 **探索模式（explore）**：還沒預設「種類」是什麼，先用 histogram 把圖片的相似關係量出來，看它自然能分出什麼，再決定分類標準。詳見 `histogram_and_clustering.md`。

## 卡住時查哪一份

| 你現在卡住的狀況 | 看這份 |
|---|---|
| 不知道下一步要做什麼、做到哪一階段了 | `project_roadmap.md` |
| histogram 怎麼設計、距離怎麼算、分群結果怪怪的 | `histogram_and_clustering.md` |
| 想加快速度、想用 goroutine、程式記憶體爆掉 | `goroutine_concurrency.md` |
| Go 語法／標準庫行為跟 Python 想的不一樣、debugger 看不到東西 | `go_pitfalls_checklist.md`（速查）與 `go_notes.md`（詳細版） |
| `go.mod`、module、Delve、`image.Image` 型別等基礎觀念 | `go_notes.md`（先前整理的入門筆記） |

## 檔案一覽

- `project_roadmap.md` — 時間線：階段規劃、每階段的產出與驗收標準、目前進度。
- `histogram_and_clustering.md` — 方法論：特徵設計、距離與分群、已知陷阱、探索階段要回答的問題。
- `goroutine_concurrency.md` — 併發：哪個階段適合平行、worker pool 模式、禁止事項與 benchmark 流程。
- `go_pitfalls_checklist.md` — 速查表：本專案會實際撞到的 Go／`image` 套件坑，寫碼前掃一遍。
- `go_notes.md` — 既有的 Go 入門筆記（從 Python 轉過來的觀念整理），不在本索引重複維護。

## 貫穿全專案的三條原則

1. **先正確，再量測，再優化。** 單線程版本先對，goroutine 和效能優化是後面的事。
2. **特徵萃取和分群調參要分開。** 特徵算完存檔，調參時重跑便宜的分群，不要每次重新解碼幾千張圖。
3. **搬移檔案前一定先 dry-run。** 先印報表、人工抽查，確認無誤才動檔案，而且要有 log 可以還原。
