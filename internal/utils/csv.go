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

func ReadKlinesFromCSV(filename string) ([]*domain.Kline, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var klines []*domain.Kline
	for i, rec := range records {
		if i == 0 {
			continue
		} // skip header
		openTime, _ := time.Parse(time.RFC3339, rec[0])
		closeTime, _ := time.Parse(time.RFC3339, rec[1])
		open, _ := strconv.ParseFloat(rec[4], 64)
		high, _ := strconv.ParseFloat(rec[5], 64)
		low, _ := strconv.ParseFloat(rec[6], 64)
		close, _ := strconv.ParseFloat(rec[7], 64)
		volume, _ := strconv.ParseFloat(rec[8], 64)
		klines = append(klines, &domain.Kline{
			OpenTime: openTime, CloseTime: closeTime, Symbol: rec[2], Interval: rec[3],
			Open: open, High: high, Low: low, Close: close, Volume: volume, IsFinal: true,
		})
	}
	return klines, nil
}

func WriteTradesToCSV(trades []*domain.Trade, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"position_id", "symbol", "entry_price", "exit_price", "quantity", "leverage", "pnl", "entry_time", "exit_time", "close_reason"})
	for _, t := range trades {
		writer.Write([]string{
			strconv.FormatInt(t.PositionID, 10),
			t.Symbol,
			strconv.FormatFloat(t.EntryPrice, 'f', -1, 64),
			strconv.FormatFloat(t.ExitPrice, 'f', -1, 64),
			strconv.FormatFloat(t.Quantity, 'f', -1, 64),
			strconv.Itoa(t.Leverage),
			strconv.FormatFloat(t.PNL, 'f', -1, 64),
			t.EntryTime.Format(time.RFC3339),
			t.ExitTime.Format(time.RFC3339),
			string(t.CloseReason),
		})
	}
	return writer.Error()
}

func ReadTradesFromCSV(filename string) ([]*domain.Trade, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var trades []*domain.Trade
	for i, rec := range records {
		if i == 0 {
			continue
		} // skip header
		positionID, _ := strconv.ParseInt(rec[0], 10, 64)
		entryPrice, _ := strconv.ParseFloat(rec[2], 64)
		exitPrice, _ := strconv.ParseFloat(rec[3], 64)
		quantity, _ := strconv.ParseFloat(rec[4], 64)
		leverage, _ := strconv.Atoi(rec[5])
		pnl, _ := strconv.ParseFloat(rec[6], 64)
		entryTime, _ := time.Parse(time.RFC3339, rec[7])
		exitTime, _ := time.Parse(time.RFC3339, rec[8])
		closeReason := rec[9]
		trades = append(trades, &domain.Trade{
			PositionID:  positionID,
			Symbol:      rec[1],
			EntryPrice:  entryPrice,
			ExitPrice:   exitPrice,
			Quantity:    quantity,
			Leverage:    leverage,
			PNL:         pnl,
			EntryTime:   entryTime,
			ExitTime:    exitTime,
			CloseReason: domain.CloseReason(closeReason),
		})
	}
	return trades, nil
}
