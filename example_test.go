package googlegeocode_test

import (
	"encoding/json"
	"fmt"
	"googlegeocode"
)

func ExampleGetResults() {
	// A set of addresses that we wish to geocode.
	addresses := []string{
		"704 S 2nd St, Minneapolis, MN 55401",
		"200, Tower Ave, St Paul, MN 55111",
		"240 Summit Ave, St Paul, MN 55102",
		"239 Selby Ave, St Paul, MN 55102",
		"75 Rev Dr Martin Luther King Jr Boulevard., St Paul, MN 55155",
	}

	// Initialize variables
	results := make([]googlegeocode.Results, len(addresses))
	var err error

	// Execute the a query for each result
	for index, address := range addresses {
		results[index], err = googlegeocode.GetResults(address)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Print the results as nicely formated JSON.
	jsonBytes, err := json.MarshalIndent(results, "", "   ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", jsonBytes)
}
