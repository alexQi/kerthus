package districtimport

import (
	"strings"
	"testing"
)

func TestCompleteHierarchy(t *testing.T) {
	rows, e := Read(strings.NewReader("id,parent_id,name\n1,0,Province\n2,1,City\n3,2,District\n"))
	if e != nil || len(rows) != 3 {
		t.Fatal(e)
	}
	for _, v := range []string{"id,parent_id,name\n1,2,A\n2,1,B\n", "id,parent_id,name\n1,9,A\n", "id,parent_id,name\n1,0,A\n1,0,B\n"} {
		if _, e := Read(strings.NewReader(v)); e == nil {
			t.Fatal("invalid hierarchy accepted")
		}
	}
}
