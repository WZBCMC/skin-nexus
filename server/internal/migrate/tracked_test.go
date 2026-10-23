package migrate

import (
	"errors"
	"testing"
)

func TestIsDuplicateCollision(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("Error 1050: Table 'skin_nexus.users' already exists"), true},
		{errors.New("Error 1060: Duplicate column name 'email_verified'"), true},
		{errors.New("Error 1061: Duplicate key name 'PRIMARY'"), true},
		// data collisions and everything else must fail loud
		{errors.New("Error 1062: Duplicate entry 'x' for key 'uniq'"), false},
		{errors.New("Error 1045: Access denied"), false},
		{errors.New("syntax error near FOR") , false},
	}
	for _, c := range cases {
		if got := isDuplicateCollision(c.err); got != c.want {
			t.Errorf("isDuplicateCollision(%q) = %v, want %v", c.err.Error(), got, c.want)
		}
	}
}
