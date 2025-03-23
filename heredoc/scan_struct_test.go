package heredoc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	// Test case 1: Simple struct with basic types
	t.Run("SimpleStruct", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
			Params: []Param{
				{Name: "name", Payload: "John"},
				{Name: "age", Payload: "30"},
				{Name: "active", Payload: "true"},
			},
		}

		type User struct {
			Name   string `json:"name"`
			Age    int    `json:"age"`
			Active bool   `json:"active"`
		}

		var user User
		err := cmd.Scan(&user)

		assert.NoError(err)
		assert.Equal("John", user.Name)
		assert.Equal(30, user.Age)
		assert.True(user.Active)
	})

	// Test case 2: Nested struct
	t.Run("NestedStruct", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
			Params: []Param{
				{Name: "name", Payload: "John"},
				{Name: "address", Payload: `{"street":"Main St","city":"New York"}`},
			},
		}

		type Address struct {
			Street string `json:"street"`
			City   string `json:"city"`
		}

		type User struct {
			Name    string  `json:"name"`
			Address Address `json:"address"`
		}

		var user User
		err := cmd.Scan(&user)

		assert.NoError(err)
		assert.Equal("John", user.Name)
		assert.Equal("Main St", user.Address.Street)
		assert.Equal("New York", user.Address.City)
	})

	// Test case 3: Slice
	t.Run("Slice", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
			Params: []Param{
				{Name: "names", Payload: `["John","Jane","Bob"]`},
			},
		}

		type Data struct {
			Names []string `json:"names"`
		}

		var data Data
		err := cmd.Scan(&data)

		assert.NoError(err)
		assert.Equal([]string{"John", "Jane", "Bob"}, data.Names)
	})

	// Test case 4: Error - target is not a pointer
	t.Run("NotPointer", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
		}

		type User struct{}

		var user User
		err := cmd.Scan(user)

		assert.Error(err)
		assert.Contains(err.Error(), "must be a non-nil pointer")
	})

	// Test case 5: Error - target is not a struct
	t.Run("NotStruct", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
		}

		var data string
		err := cmd.Scan(&data)

		assert.Error(err)
		assert.Contains(err.Error(), "must be a pointer to struct")
	})

	// Test case 6: Error - required field missing
	t.Run("RequiredField", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
			Params: []Param{
				{Name: "name", Payload: "John"},
			},
		}

		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var user User
		err := cmd.Scan(&user)

		assert.Error(err)
		assert.Contains(err.Error(), "required parameter email not found")
	})

	// Test case 7: Error - invalid value type
	t.Run("InvalidType", func(t *testing.T) {
		assert := assert.New(t)

		cmd := Command{
			Name: "test",
			Params: []Param{
				{Name: "age", Payload: "not-a-number"},
			},
		}

		type User struct {
			Age int `json:"age"`
		}

		var user User
		err := cmd.Scan(&user)

		assert.Error(err)
		assert.Contains(err.Error(), "failed to set field Age")
	})
}
