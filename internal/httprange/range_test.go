package httprange

import (
	"testing"
)

func TestParseRange(t *testing.T) {
	var tests = []struct {
		s      string
		length int64
		r      []Range
	}{
		{"", 0, nil},
		{"", 1000, nil},
		{"foo", 0, nil},
		{"bytes=", 0, nil},
		{"bytes=7", 10, nil},
		{"bytes= 7 ", 10, nil},
		{"bytes=1-", 0, nil},
		{"bytes=5-4", 10, nil},
		{"bytes=0-2,5-4", 10, nil},
		{"bytes=2-5,4-3", 10, nil},
		{"bytes=--5,4--3", 10, nil},
		{"bytes=A-", 10, nil},
		{"bytes=A- ", 10, nil},
		{"bytes=A-Z", 10, nil},
		{"bytes= -Z", 10, nil},
		{"bytes=5-Z", 10, nil},
		{"bytes=Ran-dom, garbage", 10, nil},
		{"bytes=0x01-0x02", 10, nil},
		{"bytes=         ", 10, nil},
		{"bytes= , , ,   ", 10, nil},
		{"bytes=0-9", 10, []Range{{0, 10}}},
		{"bytes=0-", 10, []Range{{0, 10}}},
		{"bytes=5-", 10, []Range{{5, 5}}},
		{"bytes=0-20", 10, []Range{{0, 10}}},
		{"bytes=15-,0-5", 10, nil},
		{"bytes=1-2,5-", 10, []Range{{1, 2}, {5, 5}}},
		{"bytes=-2 , 7-", 11, []Range{{9, 2}, {7, 4}}},
		{"bytes=0-0 ,2-2, 7-", 11, []Range{{0, 1}, {2, 1}, {7, 4}}},
		{"bytes=-5", 10, []Range{{5, 5}}},
		{"bytes=-15", 10, []Range{{0, 10}}},
		{"bytes=0-499", 10000, []Range{{0, 500}}},
		{"bytes=500-999", 10000, []Range{{500, 500}}},
		{"bytes=-500", 10000, []Range{{9500, 500}}},
		{"bytes=9500-", 10000, []Range{{9500, 500}}},
		{"bytes=0-0,-1", 10000, []Range{{0, 1}, {9999, 1}}},
		{"bytes=500-600,601-999", 10000, []Range{{500, 101}, {601, 399}}},
		{"bytes=500-700,601-999", 10000, []Range{{500, 201}, {601, 399}}},
	}

	for _, tt := range tests {
		r := tt.r
		ranges, err := ParseRange(tt.s, tt.length)
		if err != nil && r != nil {
			t.Errorf("ParseRange(%q) returned error %q", tt.s, err)
		}
		if len(ranges) != len(r) {
			t.Errorf("len(ParseRange(%q)) = %d, want %d", tt.s, len(ranges), len(r))
		}
		for i := range r {
			if ranges[i].Start != r[i].Start {
				t.Errorf("ParseRange(%q)[%d].start = %d, want %d", tt.s, i, ranges[i].Start, r[i].Start)
			}
			if ranges[i].Length != r[i].Length {
				t.Errorf("ParseRange(%q)[%d].length = %d, want %d", tt.s, i, ranges[i].Length, r[i].Length)
			}
		}
	}
}

func TestParseContentRange(t *testing.T) {
	var tests = []struct {
		s   string
		cr  *ContentRange
		err string
	}{
		{"", nil, "no Content-Range header"},
		{"bytes 500-600/*", &ContentRange{Start: 599, End: 600, Size: 0}, "cannot parse size of Content-Range header"},
		{"bytes -600/999", &ContentRange{Start: 599, End: 600, Size: 0}, "cannot parse start of Content-Range header"},
		{"bytes 0-/999", &ContentRange{Start: 599, End: 600, Size: 0}, "cannot parse end of Content-Range header"},
		{"bytes 0-63/128", &ContentRange{Start: 0, End: 63, Size: 128}, ""},
	}

	for _, tt := range tests {
		cr, err := ParseContentRange(tt.s)

		if err != nil {
			if err.Error() != tt.err {
				t.Errorf("ParseContentRange(%q) error = %s, want %s", tt.s, err, tt.err)
			}

			continue
		}

		if cr.Start != tt.cr.Start {
			t.Errorf("ParseContentRange(%q).Start = %d, want %d", tt.s, cr.Start, tt.cr.Start)
		}

		if cr.End != tt.cr.End {
			t.Errorf("ParseContentRange(%q).End = %d, want %d", tt.s, cr.End, tt.cr.End)
		}

		if cr.Size != tt.cr.Size {
			t.Errorf("ParseContentRange(%q).Size = %d, want %d", tt.s, cr.Size, tt.cr.Size)
		}
	}
}
