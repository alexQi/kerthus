// Package districtimport validates an explicit administrative-area export.
package districtimport

import (
	"encoding/csv"
	"fmt"
	"io"
	"kerthus/internal/saas/domain/dictionary"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Read(r io.Reader) ([]dictionary.District, error) {
	reader := csv.NewReader(io.LimitReader(r, 32<<20))
	header, e := reader.Read()
	if e != nil {
		return nil, e
	}
	if strings.Join(header, ",") != "id,parent_id,name" {
		return nil, fmt.Errorf("CSV header must be id,parent_id,name")
	}
	out := []dictionary.District{}
	ids := map[int64]int64{}
	for {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		id, e := strconv.ParseInt(row[0], 10, 64)
		if e != nil || id <= 0 {
			return nil, fmt.Errorf("invalid district ID")
		}
		parent, e := strconv.ParseInt(row[1], 10, 64)
		if e != nil || parent < 0 {
			return nil, fmt.Errorf("invalid parent ID")
		}
		name := strings.TrimSpace(row[2])
		if name == "" || utf8.RuneCountInString(name) > 191 {
			return nil, fmt.Errorf("invalid district name")
		}
		if _, ok := ids[id]; ok {
			return nil, fmt.Errorf("duplicate district ID %d", id)
		}
		ids[id] = parent
		out = append(out, dictionary.District{ID: id, ParentID: parent, Name: name})
		if len(out) > 100000 {
			return nil, fmt.Errorf("district export exceeds 100000 rows")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("district export is empty")
	}
	for id, parent := range ids {
		seen := map[int64]bool{id: true}
		for parent != 0 {
			if seen[parent] {
				return nil, fmt.Errorf("district parent cycle")
			}
			seen[parent] = true
			next, ok := ids[parent]
			if !ok {
				return nil, fmt.Errorf("parent %d missing from export", parent)
			}
			parent = next
		}
	}
	return out, nil
}
