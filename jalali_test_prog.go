package main

import "fmt"

func gregorianToJalali(gy, gm, gd int) (jy, jm, jd int) {
	gy -= 1600
	gm--
	gd--

	gDayNo := 365*gy + (gy+3)/4 - (gy+99)/100 + (gy+399)/400
	gDaysInMonth := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := 0; i < gm; i++ {
		gDayNo += gDaysInMonth[i]
	}
	if gm > 1 && ((gy+1600)%4 == 0 && (gy+1600)%100 != 0 || (gy+1600)%400 == 0) {
		gDayNo++
	}
	gDayNo += gd

	jDayNo := gDayNo - 79

	jNp := jDayNo / 12053
	jDayNo %= 12053

	jy = 979 + 33*jNp + 4*(jDayNo/1461)
	jDayNo %= 1461

	if jDayNo >= 366 {
		jy += (jDayNo - 1) / 365
		jDayNo = (jDayNo - 1) % 365
	}

	if jDayNo < 186 {
		jm = 1 + jDayNo/31
		jd = 1 + jDayNo%31
	} else {
		jm = 7 + (jDayNo-186)/30
		jd = 1 + (jDayNo-186)%30
	}
	return
}

func main() {
	// Known conversions:
	// 2026-03-21 -> 1405/01/01 (Nowruz 1405)
	// 2026-09-27 -> 1405/10/05
	// 2025-03-21 -> 1404/01/01
	// 2024-02-29 (leap) -> 1402/12/10
	tests := []struct {
		gy, gm, gd int
		want       string
		desc       string
	}{
		{2026, 3, 21, "1405/01/01", "Nowruz 1405"},
		{2026, 9, 27, "1405/10/05", "today"},
		{2025, 3, 21, "1404/01/01", "Nowruz 1404"},
		{2024, 2, 29, "1402/12/10", "Gregorian leap day"},
		{2026, 9, 22, "1405/10/00 ", "invalid sanity (should not match)"},
	}
	for _, tt := range tests {
		jy, jm, jd := gregorianToJalali(tt.gy, tt.gm, tt.gd)
		got := fmt.Sprintf("%04d/%02d/%02d", jy, jm, jd)
		status := "OK"
		if got != tt.want {
			status = "MISMATCH"
		}
		fmt.Printf("%s: %d-%02d-%02d -> %s (want %s) [%s]\n", tt.desc, tt.gy, tt.gm, tt.gd, got, tt.want, status)
	}
}
