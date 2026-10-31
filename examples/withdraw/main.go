package main

import (
	"context"
	"fmt"
	"log"

	regel "github.com/sodiqscript111/Regel"
)

func broken() *regel.Spec {
	return regel.New("Withdraw").
		Var("balance", 100).
		Workers("a", "b").
		Step("check", regel.If("balance >= 60").Goto("debit").Else("done")).
		Step("debit", regel.Set("balance", "balance - 60")).
		Always("NoOverdraft", "balance >= 0")
}

func fixed() *regel.Spec {
	return regel.New("WithdrawFixed").
		Var("balance", 100).
		Workers("a", "b").
		Step("withdraw", regel.Set("balance", "IF balance >= 60 THEN balance - 60 ELSE balance")).
		Always("NoOverdraft", "balance >= 0").
		Finishes()
}

func main() {
	for _, spec := range []*regel.Spec{broken(), fixed()} {
		result, err := spec.Check(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result)
		fmt.Println()
	}
}
