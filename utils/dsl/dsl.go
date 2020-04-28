package dsl

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
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
	return fmt.Sprintf("prepare(signer: AuthAccount) { %s }", p.Content.ToCadence())
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
	Address flow.Address
}

func (i Import) ToCadence() string {
	if i.Address != flow.ZeroAddress {
		return fmt.Sprintf("import 0x%s", i.Address.Short())
	}
	return ""
}

type UpdateAccountCode struct {
	Code string
}

func (u UpdateAccountCode) ToCadence() string {

	bytes := []byte(u.Code)

	hexCode := hex.EncodeToString(bytes)

	return fmt.Sprintf(`
		let code = "%s"
        signer.setCode(code.decodeHex())
    `, hexCode)
}

type Main struct {
	ReturnType string
	Code       string
}

func (m Main) ToCadence() string {
	return fmt.Sprintf("import 0x01\npub fun main(): %s { %s }", m.ReturnType, m.Code)
}

type Code string

func (c Code) ToCadence() string {
	return string(c)
}
