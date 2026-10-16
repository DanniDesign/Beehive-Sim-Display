package simulation

import "testing"

func BenchmarkYear(b *testing.B) {
	for i := 0; i < b.N; i++ {
		h := NewHiveWithSeed(int64(i), 120, 64, 32, DefaultParams())
		for t := 0; t < YearDays*DayTicks; t++ {
			h.Update()
		}
	}
}
