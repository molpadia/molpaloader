package httprange

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Range struct {
	Start  int64
	Length int64
}

type ContentRange struct {
	Start, End, Size int64
}

// Get the length the file resumed to upload.
func (cr *ContentRange) Length() int64 { return cr.End - cr.Start + 1 }

// Get the current number of times the file resumed to upload.
func (cr *ContentRange) CurrentPart() int64 { return (cr.Start / (cr.End - cr.Start)) + 1 }

// Get all parts the file resumed to upload.
func (cr *ContentRange) Parts() int64 {
	remainder := 0
	if cr.Size%cr.Length() > 0 {
		remainder = 1
	}
	return cr.Size/cr.Length() + int64(remainder)
}

// Determine whether the given byte-offset of the last byte in the range.
func (cr *ContentRange) IsLastByte() bool {
	return cr.End+1 >= cr.Size
}

func ParseRange(s string, size int64) ([]Range, error) {
	if s == "" {
		return nil, nil
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, fmt.Errorf("invalid range header %q", s)
	}
	var ranges []Range
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, fmt.Errorf("invalid range header %q", s)
		}
		var r Range
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		if start == "" {
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range header %q", s)
			}
			if i > size {
				i = size
			}
			r.Start = size - i
			r.Length = size - r.Start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i >= size || i < 0 {
				return nil, fmt.Errorf("invalid range header %q", s)
			}

			r.Start = i
			if end == "" {
				r.Length = size - r.Start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.Start > i {
					return nil, fmt.Errorf("invalid range header %q", s)
				}
				if i >= size {
					i = size - 1
				}
				r.Length = i - r.Start + 1
			}
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func ParseContentRange(s string) (*ContentRange, error) {
	const b = "bytes "
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid unit of Content-Range header")
	}
	r := strings.Split(s[len(b):], "/")
	if len(r) != 2 {
		return nil, errors.New("invalid size of Content-Range header")
	}
	size, err := strconv.ParseInt(strings.TrimSpace(r[1]), 10, 64)
	if err != nil {
		return nil, errors.New("cannot parse size of Content-Range header")
	}
	r = strings.Split(r[0], "-")
	if len(r) != 2 {
		return nil, errors.New("cannot parse Content-Range header, expected format \"start-end\"")
	}
	start, err := strconv.ParseInt(strings.TrimSpace(r[0]), 10, 64)
	if err != nil {
		return nil, errors.New("cannot parse start of Content-Range header")
	}
	end, err := strconv.ParseInt(strings.TrimSpace(r[1]), 10, 64)
	if err != nil {
		return nil, errors.New("cannot parse end of Content-Range header")
	}
	cr := &ContentRange{
		Start: start,
		End:   end,
		Size:  size,
	}
	return cr, nil
}
