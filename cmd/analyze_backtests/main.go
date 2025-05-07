package main

import (
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

func main() {
	// Find all backtest trade files
	files, err := findBacktestFiles("data", "improved_backtest_trades")
	if err != nil {
		log.Fatalf("Error finding backtest files: %v", err)
	}

	if len(files) == 0 {
		log.Println("No backtest files found. Run the improved backtest runner first.")
		return
	}

	// Create a tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "File\tTrades\tWinRate\tAvgWin\tAvgLoss\tTotalPnL\tMaxDD\tTP%\t")

	// Process each file
	for _, file := range files {
		trades, err := utils.ReadTradesFromCSV(file)
		if err != nil {
			log.Printf("Error reading trades from %s: %v", file, err)
			continue
		}

		// Calculate statistics
		stats := calculateTradeStats(trades)

		// Extract TP value from filename (e.g., improved_backtest_trades_tp1.5.csv -> 1.5)
		tp := extractTPFromFilename(file)

		// Print statistics
		fmt.Fprintf(w, "%s\t%d\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t\n",
			filepath.Base(file),
			stats.TotalTrades,
			stats.WinRate*100,
			stats.AvgWin,
			stats.AvgLoss,
			stats.TotalPnL,
			stats.MaxDrawdown,
			tp,
		)
	}
	w.Flush()

	// Print additional analysis
	fmt.Println("\n## Trend Reversal Analysis")
	analyzeTrendReversals(files)
}

// TradeStats holds statistics about a set of trades
type TradeStats struct {
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	WinRate       float64
	AvgWin        float64
	AvgLoss       float64
	TotalPnL      float64
	MaxDrawdown   float64
}

// calculateTradeStats calculates statistics for a set of trades
func calculateTradeStats(trades []*domain.Trade) TradeStats {
	var stats TradeStats
	stats.TotalTrades = len(trades)

	if stats.TotalTrades == 0 {
		return stats
	}

	// Calculate win/loss stats
	var winningPnL, losingPnL float64
	var maxBalance, currentBalance, maxDrawdown float64
	currentBalance = 1000.0 // Assume starting balance of 1000
	maxBalance = currentBalance

	for _, trade := range trades {
		stats.TotalPnL += trade.PNL
		currentBalance += trade.PNL

		// Update max balance and drawdown
		if currentBalance > maxBalance {
			maxBalance = currentBalance
		}

		drawdown := (maxBalance - currentBalance) / maxBalance
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}

		if trade.PNL > 0 {
			stats.WinningTrades++
			winningPnL += trade.PNL
		} else {
			stats.LosingTrades++
			losingPnL += trade.PNL
		}
	}

	// Calculate averages
	if stats.WinningTrades > 0 {
		stats.AvgWin = winningPnL / float64(stats.WinningTrades)
	}
	if stats.LosingTrades > 0 {
		stats.AvgLoss = losingPnL / float64(stats.LosingTrades)
	}
	stats.WinRate = float64(stats.WinningTrades) / float64(stats.TotalTrades)
	stats.MaxDrawdown = maxDrawdown

	return stats
}

// findBacktestFiles finds all backtest trade files in the specified directory
func findBacktestFiles(dir, prefix string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".csv") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	// Sort files by TP value
	sort.Slice(files, func(i, j int) bool {
		tpi := extractTPFromFilename(files[i])
		tpj := extractTPFromFilename(files[j])
		return tpi < tpj
	})

	return files, nil
}

// extractTPFromFilename extracts the TP value from a filename
// e.g., improved_backtest_trades_tp1.5.csv -> 1.5
func extractTPFromFilename(filename string) float64 {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_tp")
	if len(parts) < 2 {
		return 0
	}

	tpStr := strings.TrimSuffix(parts[1], ".csv")
	var tp float64
	fmt.Sscanf(tpStr, "%f", &tp)
	return tp
}

// analyzeTrendReversals analyzes the trend reversal exits
func analyzeTrendReversals(files []string) {
	for _, file := range files {
		trades, err := utils.ReadTradesFromCSV(file)
		if err != nil {
			log.Printf("Error reading trades from %s: %v", file, err)
			continue
		}

		// Count trades by close reason
		closeReasonCounts := make(map[domain.CloseReason]int)
		closeReasonPnL := make(map[domain.CloseReason]float64)

		for _, trade := range trades {
			closeReasonCounts[trade.CloseReason]++
			closeReasonPnL[trade.CloseReason] += trade.PNL
		}

		// Print trend reversal statistics
		fmt.Printf("\nFile: %s\n", filepath.Base(file))
		fmt.Println("Close Reason\tCount\tTotal PnL\tAvg PnL")

		// Sort reasons for consistent output
		var reasons []domain.CloseReason
		for reason := range closeReasonCounts {
			reasons = append(reasons, reason)
		}
		sort.Slice(reasons, func(i, j int) bool {
			return string(reasons[i]) < string(reasons[j])
		})

		for _, reason := range reasons {
			count := closeReasonCounts[reason]
			totalPnL := closeReasonPnL[reason]
			avgPnL := 0.0
			if count > 0 {
				avgPnL = totalPnL / float64(count)
			}

			fmt.Printf("%s\t%d\t%.2f\t%.2f\n", reason, count, totalPnL, avgPnL)
		}

		// Print additional analysis for day trading specific exit reasons
		fmt.Println("\nDay Trading Exit Analysis:")
		dayTradingExits := []domain.CloseReason{
			domain.CloseReasonVolatilityDrop,
			domain.CloseReasonConsolidation,
			domain.CloseReasonMarketClose,
		}

		dayTradingExitCount := 0
		dayTradingExitPnL := 0.0

		for _, reason := range dayTradingExits {
			count := closeReasonCounts[reason]
			totalPnL := closeReasonPnL[reason]
			dayTradingExitCount += count
			dayTradingExitPnL += totalPnL

			if count > 0 {
				fmt.Printf("%s: %d trades, PnL: %.2f, Avg: %.2f\n",
					reason, count, totalPnL, totalPnL/float64(count))
			}
		}

		if dayTradingExitCount > 0 {
			fmt.Printf("All Day Trading Exits: %d trades, PnL: %.2f, Avg: %.2f\n",
				dayTradingExitCount, dayTradingExitPnL, dayTradingExitPnL/float64(dayTradingExitCount))
		} else {
			fmt.Println("No day trading specific exits found")
		}
	}
}
