package anomaly

// TimeSeries stores per-second counts in a fixed-length ring of buckets.
type TimeSeries struct {
	buckets []int
	cap     int
	offset  int // second value stored at buckets[0]
	len     int // number of valid seconds ending at offset+len-1
}

// NewTimeSeries creates a series that holds `capacity` one-second buckets.
func NewTimeSeries(capacity int) *TimeSeries {
	if capacity <= 0 {
		capacity = 1
	}
	return &TimeSeries{buckets: make([]int, capacity), cap: capacity}
}

// Add increments the bucket for the given second.
func (s *TimeSeries) Add(second, delta int) {
	idx := s.ensure(second)
	if idx >= 0 {
		s.buckets[idx] += delta
	}
}

// Sum returns the total count for buckets in [secondFrom, secondTo] inclusive.
func (s *TimeSeries) Sum(secondFrom, secondTo int) int {
	total := 0
	for sec := secondFrom; sec <= secondTo; sec++ {
		if idx := s.indexFor(sec); idx >= 0 {
			total += s.buckets[idx]
		}
	}
	return total
}

func (s *TimeSeries) ensure(second int) int {
	if s.len == 0 {
		s.offset = second
		s.len = 1
		s.buckets[0] = 0
		return 0
	}
	end := s.offset + s.len - 1
	if second < s.offset {
		return -1
	}
	if second <= end {
		return s.mod(second - s.offset)
	}

	// second > end: expand or shift the window to include it.
	if second < s.offset+s.cap {
		// Normal advance within capacity: zero newly visible buckets.
		for sec := end + 1; sec <= second; sec++ {
			idx := s.mod(sec - s.offset)
			s.buckets[idx] = 0
		}
		s.len = second - s.offset + 1
		if s.len > s.cap {
			// Window grew beyond capacity; drop oldest seconds.
			s.offset += s.len - s.cap
			s.len = s.cap
		}
		return s.mod(second - s.offset)
	}

	// Big jump beyond current capacity: retain only the last `cap` seconds.
	newOffset := second - s.cap + 1
	newBuckets := make([]int, s.cap)
	for sec := newOffset; sec <= second; sec++ {
		idx := (sec - newOffset) % s.cap
		if sec >= s.offset && sec <= end {
			oldIdx := s.mod(sec - s.offset)
			newBuckets[idx] = s.buckets[oldIdx]
		}
	}
	s.buckets = newBuckets
	s.offset = newOffset
	s.len = s.cap
	return s.mod(second - newOffset)
}

func (s *TimeSeries) indexFor(second int) int {
	if s.len == 0 || second < s.offset || second > s.offset+s.len-1 {
		return -1
	}
	return s.mod(second - s.offset)
}

func (s *TimeSeries) mod(v int) int {
	v %= s.cap
	if v < 0 {
		v += s.cap
	}
	return v
}

// Reset clears all buckets.
func (s *TimeSeries) Reset() {
	for i := range s.buckets {
		s.buckets[i] = 0
	}
	s.offset = 0
	s.len = 0
}
