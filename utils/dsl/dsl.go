package dsl

import (
	"encoding/hex"
	"fmt"
	"strings"

	sdk "github.com/onflow/flow-go-sdk"
)

type CadenceCode interface {
	ToCadence() string
}

type Transaction struct {
	Import  Import
	Content CadenceCode
}

func (t Transaction) ToCadence() string {
	return fmt.Sprintf("%s \n transaction { %s }", t.Import.ToCadence(), t.Content.ToCadence())
}

type Prepare struct {
	Content CadenceCode
}

func (p Prepare) ToCadence() string {
	return fmt.Sprintf("prepare(signer: auth(Storage, Capabilities, Contracts) &Account) { %s }", p.Content.ToCadence())
}

type Contract struct {
	Name    string
	Members []CadenceCode
}

func (c Contract) ToCadence() string {

	memberStrings := make([]string, len(c.Members))
	for i, member := range c.Members {
		memberStrings[i] = member.ToCadence()
	}

	return fmt.Sprintf("access(all) contract %s { %s }", c.Name, strings.Join(memberStrings, "\n"))
}

type Resource struct {
	Name string
	Code string
}

func (r Resource) ToCadence() string {
	return fmt.Sprintf("access(all) resource %s { %s }", r.Name, r.Code)
}

type Import struct {
	Names   []string
	Address sdk.Address
}

func (i Import) ToCadence() string {
	if i.Address != sdk.EmptyAddress {
		if len(i.Names) > 0 {
			return fmt.Sprintf("import %s from 0x%s\n", strings.Join(i.Names, ", "), i.Address)
		}
		return fmt.Sprintf("import 0x%s\n", i.Address)
	}
	return ""
}

type UpdateAccountCode struct {
	Code string
	Name string
}

func (u UpdateAccountCode) ToCadence() string {

	bytes := []byte(u.Code)

	hexCode := hex.EncodeToString(bytes)

	return fmt.Sprintf(`
		let code = "%s"
        signer.contracts.add(name: "%s", code: code.decodeHex())
    `, hexCode, u.Name)
}

type Main struct {
	Import     Import
	ReturnType string
	Code       string
}

func (m Main) ToCadence() string {
	return fmt.Sprintf("%s \naccess(all) fun main(): %s { %s }", m.Import.ToCadence(), m.ReturnType, m.Code)
}

type Code string

func (c Code) ToCadence() string {
	return string(c)
}
