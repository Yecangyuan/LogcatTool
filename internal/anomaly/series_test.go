package anomaly

import "testing"

func TestTimeSeriesSumAndShift(t *testing.T) {
	s := NewTimeSeries(5)
	for i := 0; i < 3; i++ {
		s.Add(i, 1)
	}
	if got := s.Sum(0, 2); got != 3 {
		t.Fatalf("sum want 3 got %d", got)
	}
	s.Add(5, 10)
	if got := s.Sum(1, 5); got != 12 {
		t.Fatalf("sum after shift want 12 got %d", got)
	}
	if got := s.Sum(0, 4); got != 2 {
		t.Fatalf("sum excluding out-of-window second want 2 got %d", got)
	}
}

func TestTimeSeriesJumpBeyondCapacity(t *testing.T) {
	s := NewTimeSeries(3)
	s.Add(0, 1)
	s.Add(1, 2)
	s.Add(10, 5)
	if got := s.Sum(8, 10); got != 5 {
		t.Fatalf("want 5 after jump, got %d", got)
	}
}
