package utils

import (
	"cryptoMegaBot/internal/domain"
	"encoding/csv"
	"os"
	"strconv"
	"time"
)

func WriteKlinesToCSV(klines []*domain.Kline, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"open_time", "close_time", "symbol", "interval", "open", "high", "low", "close", "volume"})

	for _, k := range klines {
		writer.Write([]string{
			k.OpenTime.Format(time.RFC3339),
			k.CloseTime.Format(time.RFC3339),
			k.Symbol,
			k.Interval,
			strconv.FormatFloat(k.Open, 'f', -1, 64),
			strconv.FormatFloat(k.High, 'f', -1, 64),
			strconv.FormatFloat(k.Low, 'f', -1, 64),
			strconv.FormatFloat(k.Close, 'f', -1, 64),
			strconv.FormatFloat(k.Volume, 'f', -1, 64),
		})
	}
	return writer.Error()
}
