package resource

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	Byte = "{Byte}"
	KB   = "{KB}"
	MB   = "{MB}"
	GB   = "{GB}"
	TB   = "{TB}"
)

type smap struct {
	k string
	v int64
}

type Size struct {
	total    int64
	piece    [5]smap
	maxIndex int
}

func (s *Size) hasMaxKey() bool {
	return s.piece[s.maxIndex].k != ""
}

func (s *Size) split() {
	size := s.total
	for i, k := range []string{Byte, KB, MB, GB, TB} {
		p := size % 1024
		if p != 0 {
			s.maxIndex = i
		}
		s.piece[i] = smap{k, p}
		size /= 1024
	}
}

func (s *Size) Set(size int64) {
	if s.total != size {
		s.total = size
		if s.hasMaxKey() {
			s.split()
		}
	}
}

func (s *Size) Add(size int64) {
	s.Set(s.total + size)
}

func (s Size) Format(f string, limit ...string) string {
	mins, maxs := Byte, TB
	if len(limit) > 0 {
		mins = limit[0]
		if len(limit) > 1 {
			maxs = limit[1]
		}
	}

	final := make([]float64, 5)
	var temp int64
	var i int
	for i = 0; i <= s.maxIndex; i++ {
		m := s.piece[i]
		if m.k != mins {
			temp += m.v << (10 * i)
			continue
		}
		final[i] = float64(temp)/float64(int(1)<<(10*i)) + float64(m.v)
		break
	}

	var j int
	for j = i; j <= s.maxIndex; j++ {
		n := s.piece[j]
		if i != j {
			final[j] = float64(n.v)
		}
		if n.k == maxs {
			break
		}
	}
	for k := j + 1; k <= s.maxIndex; k++ {
		final[j] += float64(s.piece[k].v << (10 * (k - j)))
	}

	oldnew := make([]string, 0, 2*len(final))
	oldnew = append(oldnew, s.piece[0].k, strconv.FormatFloat(final[0], 'f', 0, 64))
	for i, p := range s.piece[1:] {
		oldnew = append(oldnew, p.k, decimal.NewFromFloat(final[i+1]).RoundFloor(2).String())
	}
	return strings.NewReplacer(oldnew...).Replace(f)
}

func (s Size) Base(base string) string {
	return s.Format(base+" "+base[1:len(base)-1], base, base)
}

func (s Size) Byte() int64 {
	return s.total
}

func (s Size) String() string {
	if !s.hasMaxKey() {
		s.split()
	}
	return s.Base(s.piece[s.maxIndex].k)
}
