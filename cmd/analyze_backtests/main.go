package main

import (
	"cryptoMegaBot/internal/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	files, err := os.ReadDir("data")
	if err != nil {
		fmt.Println("Error reading data dir:", err)
		os.Exit(1)
	}

	fmt.Printf("%-22s %-8s %-8s %-8s %-8s %-10s %-8s %-8s\n", "File", "Trades", "WinRate", "AvgWin", "AvgLoss", "TotalPnL", "MaxDD", "TP%")
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "backtest_trades_tp") || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}
		trades, err := utils.ReadTradesFromCSV(filepath.Join("data", file.Name()))
		if err != nil {
			fmt.Println("Error reading", file.Name(), ":", err)
			continue
		}
		total, wins, losses := 0, 0, 0
		totalPnL, winSum, lossSum := 0.0, 0.0, 0.0
		maxDD, balance, peak := 0.0, 0.0, 0.0
		tpCount := 0
		for _, t := range trades {
			total++
			totalPnL += t.PNL
			balance += t.PNL
			if balance > peak {
				peak = balance
			}
			dd := (peak - balance)
			if dd > maxDD {
				maxDD = dd
			}
			if t.PNL > 0 {
				wins++
				winSum += t.PNL
			} else if t.PNL < 0 {
				losses++
				lossSum += t.PNL
			}
			if strings.Contains(strings.ToLower(string(t.CloseReason)), "take") {
				tpCount++
			}
		}
		winRate := 0.0
		if total > 0 {
			winRate = float64(wins) / float64(total)
		}
		avgWin := 0.0
		if wins > 0 {
			avgWin = winSum / float64(wins)
		}
		avgLoss := 0.0
		if losses > 0 {
			avgLoss = lossSum / float64(losses)
		}
		tpPerc := 0.0
		if total > 0 {
			tpPerc = float64(tpCount) / float64(total) * 100
		}
		fmt.Printf("%-22s %-8d %-8.2f %-8.2f %-8.2f %-10.2f %-8.2f %-8.2f\n",
			file.Name(), total, winRate*100, avgWin, avgLoss, totalPnL, maxDD, tpPerc)
	}
}
