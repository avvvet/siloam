package bot

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"
	"time"

	"github.com/avvvet/siloam/db"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const additionalFeeNote = "800 Birr is for water pump and staircase light electricity"

// calculateBill computes each unit's bill share from readings and previous readings
func calculateBill(readings map[string]*db.Reading, previous map[string]int, totalBill float64, additionalFee float64) *db.Bill {
	bill := &db.Bill{
		TotalBill:     totalBill,
		AdditionalFee: additionalFee,
		Units:         make(map[string]float64),
		Diffs:         make(map[string]int),
		Percents:      make(map[string]float64),
		Previous:      make(map[string]int),
		Current:       make(map[string]int),
	}

	totalUsage := 0
	for _, unit := range allUnits {
		r, ok := readings[unit]
		if !ok {
			continue
		}
		prev := previous[unit]
		diff := r.Value - prev
		if diff < 0 {
			diff = 0
		}
		bill.Previous[unit] = prev
		bill.Current[unit] = r.Value
		bill.Diffs[unit] = diff
		totalUsage += diff
	}

	for _, unit := range allUnits {
		if totalUsage == 0 {
			bill.Units[unit] = 0
			bill.Percents[unit] = 0
			continue
		}
		diff := bill.Diffs[unit]
		pct := float64(diff) / float64(totalUsage) * 100
		amount := pct / 100 * totalBill
		bill.Percents[unit] = math.Round(pct*100) / 100
		bill.Units[unit] = math.Round(amount*100) / 100
	}

	return bill
}

// generateBillImage creates a PNG bill image and returns the file path
func generateBillImage(bill *db.Bill, creatorUsername string) (string, error) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)
	monthStr := now.Format("January 2006")
	dateStr := now.Format("January 02, 2006")

	additionalFee := bill.AdditionalFee
	totalAdditional := additionalFee * 16
	grandTotal := bill.TotalBill + totalAdditional

	// Image dimensions
	width := 1000
	rowHeight := 30
	headerRows := 5
	tableRows := len(allUnits) + 2
	noteRows := 2
	footerRows := 2
	height := (headerRows+tableRows+noteRows+footerRows)*rowHeight + 80

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Colors
	bgWhite := color.RGBA{255, 255, 255, 255}
	headerBg := color.RGBA{80, 80, 80, 255}
	tableHeaderBg := color.RGBA{110, 110, 110, 255}
	rowEven := color.RGBA{235, 235, 235, 255}
	rowOdd := color.RGBA{255, 255, 255, 255}
	textDark := color.RGBA{30, 30, 30, 255}
	textWhite := color.RGBA{255, 255, 255, 255}
	noteBg := color.RGBA{255, 248, 220, 255}
	footerBg := color.RGBA{245, 245, 245, 255}
	borderColor := color.RGBA{190, 210, 235, 255}
	totalRowBg := color.RGBA{80, 80, 80, 255}

	// Background
	draw.Draw(img, img.Bounds(), &image.Uniform{bgWhite}, image.Point{}, draw.Src)

	face := basicfont.Face7x13

	drawText := func(x, y int, c color.Color, text string) {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(text)
	}

	drawTextCentered := func(y int, c color.Color, text string) {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(c),
			Face: face,
		}
		w := d.MeasureString(text).Ceil()
		x := (width - w) / 2
		d.Dot = fixed.P(x, y)
		d.DrawString(text)
	}

	fillRect := func(x, y, w, h int, c color.Color) {
		for row := y; row < y+h; row++ {
			for col := x; col < x+w; col++ {
				img.Set(col, row, c)
			}
		}
	}

	drawHLine := func(x, y, w int, c color.Color) {
		for col := x; col < x+w; col++ {
			img.Set(col, y, c)
		}
	}

	y := 10

	// ── Header ──
	fillRect(0, y, width, rowHeight*headerRows, headerBg)
	drawTextCentered(y+20, textWhite, "WATER BILL REPORT")
	drawTextCentered(y+40, textWhite, strings.ToUpper(monthStr))
	drawTextCentered(y+60, textWhite, fmt.Sprintf("Date: %s", dateStr))
	drawTextCentered(y+80, textWhite, fmt.Sprintf("Water Bill: %.0f Birr   |   Pump & Electricity: %.0f Birr", bill.TotalBill, totalAdditional))
	drawTextCentered(y+100, textWhite, fmt.Sprintf("Grand Total: %.0f Birr", grandTotal))
	y += rowHeight*headerRows + 10

	// ── Table header ──
	cols := []string{"House", "Previous", "Current", "Diff", "%", "Water Share", "Additional", "Total"}
	colX := []int{10, 90, 190, 290, 370, 460, 590, 720}

	fillRect(0, y, width, rowHeight, tableHeaderBg)
	for i, col := range cols {
		drawText(colX[i]+4, y+20, textWhite, col)
	}
	y += rowHeight

	// ── Unit rows ──
	var grandTotalAmount float64
	for idx, unit := range allUnits {
		bg := rowOdd
		if idx%2 == 0 {
			bg = rowEven
		}
		fillRect(0, y, width, rowHeight, bg)
		drawHLine(0, y, width, borderColor)

		prev := bill.Previous[unit]
		curr := bill.Current[unit]
		diff := bill.Diffs[unit]
		pct := bill.Percents[unit]
		waterShare := bill.Units[unit]
		total := waterShare + additionalFee
		grandTotalAmount += total

		drawText(colX[0]+4, y+20, textDark, strings.ToUpper(unit))
		drawText(colX[1]+4, y+20, textDark, fmt.Sprintf("%d", prev))
		drawText(colX[2]+4, y+20, textDark, fmt.Sprintf("%d", curr))
		drawText(colX[3]+4, y+20, textDark, fmt.Sprintf("%d", diff))
		drawText(colX[4]+4, y+20, textDark, fmt.Sprintf("%.2f%%", pct))
		drawText(colX[5]+4, y+20, textDark, fmt.Sprintf("%.0f Birr", waterShare))
		drawText(colX[6]+4, y+20, textDark, fmt.Sprintf("%.0f Birr", additionalFee))
		drawText(colX[7]+4, y+20, textDark, fmt.Sprintf("%.0f Birr", total))
		y += rowHeight
	}

	// ── Total row ──
	fillRect(0, y, width, rowHeight, totalRowBg)
	drawText(colX[0]+4, y+20, textWhite, "TOTAL")
	drawText(colX[7]+4, y+20, textWhite, fmt.Sprintf("%.0f Birr", grandTotalAmount))
	y += rowHeight + 10

	// ── Note section (middle) ──
	fillRect(0, y, width, rowHeight*noteRows, noteBg)
	drawHLine(0, y, width, borderColor)
	drawTextCentered(y+20, textDark, "📌 Note:")
	drawTextCentered(y+40, textDark, additionalFeeNote)
	y += rowHeight*noteRows + 10

	// ── Footer ──
	fillRect(0, y, width, rowHeight*footerRows, footerBg)
	drawHLine(0, y, width, borderColor)
	// smaller feel using spacing
	drawTextCentered(y+18, color.RGBA{100, 100, 100, 255},
		"Report is generated by automated software deployed and hosted on generously provided U.S based cloud server provider.")
	drawTextCentered(y+34, color.RGBA{100, 100, 100, 255},
		fmt.Sprintf("Manager: Siloam  |  Creator: %s", creatorUsername))

	// Save to temp file
	path := fmt.Sprintf("/tmp/bill_%s.png", now.Format("2006-01"))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return "", err
	}

	return path, nil
}
