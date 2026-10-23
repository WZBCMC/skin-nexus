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
		// real go-sql-driver format carries the SQLSTATE: "Error 1050 (42S01): …"
		{errors.New("Error 1050 (42S01): Table 'users' already exists"), true},
		{errors.New("Error 1060 (42S21): Duplicate column name 'email_verified'"), true},
		{errors.New("Error 1061 (42000): Duplicate key name 'PRIMARY'"), true},
		{errors.New("Error 1050: Table 'x' already exists"), true},
		// data collisions and everything else must fail loud
		{errors.New("Error 1062 (23000): Duplicate entry 'x' for key 'uniq'"), false},
		{errors.New("Error 1045 (28000): Access denied"), false},
		{errors.New("syntax error near FOR"), false},
	}
	for _, c := range cases {
		if got := isDuplicateCollision(c.err); got != c.want {
			t.Errorf("isDuplicateCollision(%q) = %v, want %v", c.err.Error(), got, c.want)
		}
	}
}
