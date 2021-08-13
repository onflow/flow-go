package testutil

import "github.com/onflow/flow-go/model/flow"

func DeployCheckerHeavyTransaction(authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	return CreateContractDeploymentTransaction("CheckerHeavy", CheckerHeavyTransaction, authorizer, chain)
}

const CheckerHeavyTransaction = `
pub contract CheckerHeavy {

            pub resource Resource0 {
                pub let c: UInt64 
                pub let r: @Resource1

                init(_ r: @Resource1) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource0{
                    return <- create Resource0(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource1 {
                pub let c: UInt64 
                pub let r: @Resource2

                init(_ r: @Resource2) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource1{
                    return <- create Resource1(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource2 {
                pub let c: UInt64 
                pub let r: @Resource3

                init(_ r: @Resource3) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource2{
                    return <- create Resource2(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource3 {
                pub let c: UInt64 
                pub let r: @Resource4

                init(_ r: @Resource4) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource3{
                    return <- create Resource3(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource4 {
                pub let c: UInt64 
                pub let r: @Resource5

                init(_ r: @Resource5) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource4{
                    return <- create Resource4(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource5 {
                pub let c: UInt64 
                pub let r: @Resource6

                init(_ r: @Resource6) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource5{
                    return <- create Resource5(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource6 {
                pub let c: UInt64 
                pub let r: @Resource7

                init(_ r: @Resource7) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource6{
                    return <- create Resource6(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource7 {
                pub let c: UInt64 
                pub let r: @Resource8

                init(_ r: @Resource8) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource7{
                    return <- create Resource7(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource8 {
                pub let c: UInt64 
                pub let r: @Resource9

                init(_ r: @Resource9) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource8{
                    return <- create Resource8(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource9 {
                pub let c: UInt64 
                pub let r: @Resource10

                init(_ r: @Resource10) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource9{
                    return <- create Resource9(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource10 {
                pub let c: UInt64 
                pub let r: @Resource11

                init(_ r: @Resource11) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource10{
                    return <- create Resource10(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource11 {
                pub let c: UInt64 
                pub let r: @Resource12

                init(_ r: @Resource12) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource11{
                    return <- create Resource11(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource12 {
                pub let c: UInt64 
                pub let r: @Resource13

                init(_ r: @Resource13) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource12{
                    return <- create Resource12(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource13 {
                pub let c: UInt64 
                pub let r: @Resource14

                init(_ r: @Resource14) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource13{
                    return <- create Resource13(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource14 {
                pub let c: UInt64 
                pub let r: @Resource15

                init(_ r: @Resource15) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource14{
                    return <- create Resource14(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource15 {
                pub let c: UInt64 
                pub let r: @Resource16

                init(_ r: @Resource16) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource15{
                    return <- create Resource15(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource16 {
                pub let c: UInt64 
                pub let r: @Resource17

                init(_ r: @Resource17) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource16{
                    return <- create Resource16(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource17 {
                pub let c: UInt64 
                pub let r: @Resource18

                init(_ r: @Resource18) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource17{
                    return <- create Resource17(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource18 {
                pub let c: UInt64 
                pub let r: @Resource19

                init(_ r: @Resource19) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource18{
                    return <- create Resource18(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource19 {
                pub let c: UInt64 
                pub let r: @Resource20

                init(_ r: @Resource20) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource19{
                    return <- create Resource19(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource20 {
                pub let c: UInt64 
                pub let r: @Resource21

                init(_ r: @Resource21) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource20{
                    return <- create Resource20(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource21 {
                pub let c: UInt64 
                pub let r: @Resource22

                init(_ r: @Resource22) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource21{
                    return <- create Resource21(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource22 {
                pub let c: UInt64 
                pub let r: @Resource23

                init(_ r: @Resource23) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource22{
                    return <- create Resource22(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource23 {
                pub let c: UInt64 
                pub let r: @Resource24

                init(_ r: @Resource24) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource23{
                    return <- create Resource23(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource24 {
                pub let c: UInt64 
                pub let r: @Resource25

                init(_ r: @Resource25) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource24{
                    return <- create Resource24(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource25 {
                pub let c: UInt64 
                pub let r: @Resource26

                init(_ r: @Resource26) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource25{
                    return <- create Resource25(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource26 {
                pub let c: UInt64 
                pub let r: @Resource27

                init(_ r: @Resource27) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource26{
                    return <- create Resource26(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource27 {
                pub let c: UInt64 
                pub let r: @Resource28

                init(_ r: @Resource28) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource27{
                    return <- create Resource27(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource28 {
                pub let c: UInt64 
                pub let r: @Resource29

                init(_ r: @Resource29) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource28{
                    return <- create Resource28(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource29 {
                pub let c: UInt64 
                pub let r: @Resource30

                init(_ r: @Resource30) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource29{
                    return <- create Resource29(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource30 {
                pub let c: UInt64 
                pub let r: @Resource31

                init(_ r: @Resource31) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource30{
                    return <- create Resource30(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource31 {
                pub let c: UInt64 
                pub let r: @Resource32

                init(_ r: @Resource32) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource31{
                    return <- create Resource31(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource32 {
                pub let c: UInt64 
                pub let r: @Resource33

                init(_ r: @Resource33) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource32{
                    return <- create Resource32(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource33 {
                pub let c: UInt64 
                pub let r: @Resource34

                init(_ r: @Resource34) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource33{
                    return <- create Resource33(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource34 {
                pub let c: UInt64 
                pub let r: @Resource35

                init(_ r: @Resource35) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource34{
                    return <- create Resource34(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource35 {
                pub let c: UInt64 
                pub let r: @Resource36

                init(_ r: @Resource36) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource35{
                    return <- create Resource35(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource36 {
                pub let c: UInt64 
                pub let r: @Resource37

                init(_ r: @Resource37) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource36{
                    return <- create Resource36(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource37 {
                pub let c: UInt64 
                pub let r: @Resource38

                init(_ r: @Resource38) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource37{
                    return <- create Resource37(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource38 {
                pub let c: UInt64 
                pub let r: @Resource39

                init(_ r: @Resource39) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource38{
                    return <- create Resource38(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource39 {
                pub let c: UInt64 
                pub let r: @Resource40

                init(_ r: @Resource40) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource39{
                    return <- create Resource39(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource40 {
                pub let c: UInt64 
                pub let r: @Resource41

                init(_ r: @Resource41) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource40{
                    return <- create Resource40(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource41 {
                pub let c: UInt64 
                pub let r: @Resource42

                init(_ r: @Resource42) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource41{
                    return <- create Resource41(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource42 {
                pub let c: UInt64 
                pub let r: @Resource43

                init(_ r: @Resource43) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource42{
                    return <- create Resource42(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource43 {
                pub let c: UInt64 
                pub let r: @Resource44

                init(_ r: @Resource44) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource43{
                    return <- create Resource43(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource44 {
                pub let c: UInt64 
                pub let r: @Resource45

                init(_ r: @Resource45) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource44{
                    return <- create Resource44(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource45 {
                pub let c: UInt64 
                pub let r: @Resource46

                init(_ r: @Resource46) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource45{
                    return <- create Resource45(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource46 {
                pub let c: UInt64 
                pub let r: @Resource47

                init(_ r: @Resource47) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource46{
                    return <- create Resource46(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource47 {
                pub let c: UInt64 
                pub let r: @Resource48

                init(_ r: @Resource48) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource47{
                    return <- create Resource47(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource48 {
                pub let c: UInt64 
                pub let r: @Resource49

                init(_ r: @Resource49) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource48{
                    return <- create Resource48(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource49 {
                pub let c: UInt64 
                pub let r: @Resource50

                init(_ r: @Resource50) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource49{
                    return <- create Resource49(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource50 {
                pub let c: UInt64 
                pub let r: @Resource51

                init(_ r: @Resource51) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource50{
                    return <- create Resource50(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource51 {
                pub let c: UInt64 
                pub let r: @Resource52

                init(_ r: @Resource52) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource51{
                    return <- create Resource51(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource52 {
                pub let c: UInt64 
                pub let r: @Resource53

                init(_ r: @Resource53) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource52{
                    return <- create Resource52(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource53 {
                pub let c: UInt64 
                pub let r: @Resource54

                init(_ r: @Resource54) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource53{
                    return <- create Resource53(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource54 {
                pub let c: UInt64 
                pub let r: @Resource55

                init(_ r: @Resource55) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource54{
                    return <- create Resource54(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource55 {
                pub let c: UInt64 
                pub let r: @Resource56

                init(_ r: @Resource56) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource55{
                    return <- create Resource55(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource56 {
                pub let c: UInt64 
                pub let r: @Resource57

                init(_ r: @Resource57) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource56{
                    return <- create Resource56(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource57 {
                pub let c: UInt64 
                pub let r: @Resource58

                init(_ r: @Resource58) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource57{
                    return <- create Resource57(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource58 {
                pub let c: UInt64 
                pub let r: @Resource59

                init(_ r: @Resource59) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource58{
                    return <- create Resource58(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource59 {
                pub let c: UInt64 
                pub let r: @Resource60

                init(_ r: @Resource60) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource59{
                    return <- create Resource59(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource60 {
                pub let c: UInt64 
                pub let r: @Resource61

                init(_ r: @Resource61) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource60{
                    return <- create Resource60(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource61 {
                pub let c: UInt64 
                pub let r: @Resource62

                init(_ r: @Resource62) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource61{
                    return <- create Resource61(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource62 {
                pub let c: UInt64 
                pub let r: @Resource63

                init(_ r: @Resource63) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource62{
                    return <- create Resource62(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource63 {
                pub let c: UInt64 
                pub let r: @Resource64

                init(_ r: @Resource64) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource63{
                    return <- create Resource63(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource64 {
                pub let c: UInt64 
                pub let r: @Resource65

                init(_ r: @Resource65) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource64{
                    return <- create Resource64(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource65 {
                pub let c: UInt64 
                pub let r: @Resource66

                init(_ r: @Resource66) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource65{
                    return <- create Resource65(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource66 {
                pub let c: UInt64 
                pub let r: @Resource67

                init(_ r: @Resource67) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource66{
                    return <- create Resource66(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource67 {
                pub let c: UInt64 
                pub let r: @Resource68

                init(_ r: @Resource68) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource67{
                    return <- create Resource67(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource68 {
                pub let c: UInt64 
                pub let r: @Resource69

                init(_ r: @Resource69) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource68{
                    return <- create Resource68(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource69 {
                pub let c: UInt64 
                pub let r: @Resource70

                init(_ r: @Resource70) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource69{
                    return <- create Resource69(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource70 {
                pub let c: UInt64 
                pub let r: @Resource71

                init(_ r: @Resource71) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource70{
                    return <- create Resource70(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource71 {
                pub let c: UInt64 
                pub let r: @Resource72

                init(_ r: @Resource72) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource71{
                    return <- create Resource71(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource72 {
                pub let c: UInt64 
                pub let r: @Resource73

                init(_ r: @Resource73) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource72{
                    return <- create Resource72(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource73 {
                pub let c: UInt64 
                pub let r: @Resource74

                init(_ r: @Resource74) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource73{
                    return <- create Resource73(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource74 {
                pub let c: UInt64 
                pub let r: @Resource75

                init(_ r: @Resource75) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource74{
                    return <- create Resource74(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource75 {
                pub let c: UInt64 
                pub let r: @Resource76

                init(_ r: @Resource76) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource75{
                    return <- create Resource75(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource76 {
                pub let c: UInt64 
                pub let r: @Resource77

                init(_ r: @Resource77) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource76{
                    return <- create Resource76(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource77 {
                pub let c: UInt64 
                pub let r: @Resource78

                init(_ r: @Resource78) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource77{
                    return <- create Resource77(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource78 {
                pub let c: UInt64 
                pub let r: @Resource79

                init(_ r: @Resource79) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource78{
                    return <- create Resource78(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource79 {
                pub let c: UInt64 
                pub let r: @Resource80

                init(_ r: @Resource80) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource79{
                    return <- create Resource79(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource80 {
                pub let c: UInt64 
                pub let r: @Resource81

                init(_ r: @Resource81) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource80{
                    return <- create Resource80(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource81 {
                pub let c: UInt64 
                pub let r: @Resource82

                init(_ r: @Resource82) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource81{
                    return <- create Resource81(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource82 {
                pub let c: UInt64 
                pub let r: @Resource83

                init(_ r: @Resource83) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource82{
                    return <- create Resource82(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource83 {
                pub let c: UInt64 
                pub let r: @Resource84

                init(_ r: @Resource84) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource83{
                    return <- create Resource83(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource84 {
                pub let c: UInt64 
                pub let r: @Resource85

                init(_ r: @Resource85) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource84{
                    return <- create Resource84(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource85 {
                pub let c: UInt64 
                pub let r: @Resource86

                init(_ r: @Resource86) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource85{
                    return <- create Resource85(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource86 {
                pub let c: UInt64 
                pub let r: @Resource87

                init(_ r: @Resource87) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource86{
                    return <- create Resource86(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource87 {
                pub let c: UInt64 
                pub let r: @Resource88

                init(_ r: @Resource88) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource87{
                    return <- create Resource87(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource88 {
                pub let c: UInt64 
                pub let r: @Resource89

                init(_ r: @Resource89) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource88{
                    return <- create Resource88(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource89 {
                pub let c: UInt64 
                pub let r: @Resource90

                init(_ r: @Resource90) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource89{
                    return <- create Resource89(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource90 {
                pub let c: UInt64 
                pub let r: @Resource91

                init(_ r: @Resource91) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource90{
                    return <- create Resource90(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource91 {
                pub let c: UInt64 
                pub let r: @Resource92

                init(_ r: @Resource92) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource91{
                    return <- create Resource91(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource92 {
                pub let c: UInt64 
                pub let r: @Resource93

                init(_ r: @Resource93) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource92{
                    return <- create Resource92(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource93 {
                pub let c: UInt64 
                pub let r: @Resource94

                init(_ r: @Resource94) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource93{
                    return <- create Resource93(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource94 {
                pub let c: UInt64 
                pub let r: @Resource95

                init(_ r: @Resource95) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource94{
                    return <- create Resource94(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource95 {
                pub let c: UInt64 
                pub let r: @Resource96

                init(_ r: @Resource96) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource95{
                    return <- create Resource95(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource96 {
                pub let c: UInt64 
                pub let r: @Resource97

                init(_ r: @Resource97) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource96{
                    return <- create Resource96(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource97 {
                pub let c: UInt64 
                pub let r: @Resource98

                init(_ r: @Resource98) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource97{
                    return <- create Resource97(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource98 {
                pub let c: UInt64 
                pub let r: @Resource99

                init(_ r: @Resource99) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource98{
                    return <- create Resource98(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource99 {
                pub let c: UInt64 
                pub let r: @Resource100

                init(_ r: @Resource100) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource99{
                    return <- create Resource99(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource100 {
                pub let c: UInt64 
                pub let r: @Resource101

                init(_ r: @Resource101) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource100{
                    return <- create Resource100(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource101 {
                pub let c: UInt64 
                pub let r: @Resource102

                init(_ r: @Resource102) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource101{
                    return <- create Resource101(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource102 {
                pub let c: UInt64 
                pub let r: @Resource103

                init(_ r: @Resource103) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource102{
                    return <- create Resource102(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource103 {
                pub let c: UInt64 
                pub let r: @Resource104

                init(_ r: @Resource104) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource103{
                    return <- create Resource103(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource104 {
                pub let c: UInt64 
                pub let r: @Resource105

                init(_ r: @Resource105) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource104{
                    return <- create Resource104(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource105 {
                pub let c: UInt64 
                pub let r: @Resource106

                init(_ r: @Resource106) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource105{
                    return <- create Resource105(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource106 {
                pub let c: UInt64 
                pub let r: @Resource107

                init(_ r: @Resource107) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource106{
                    return <- create Resource106(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource107 {
                pub let c: UInt64 
                pub let r: @Resource108

                init(_ r: @Resource108) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource107{
                    return <- create Resource107(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource108 {
                pub let c: UInt64 
                pub let r: @Resource109

                init(_ r: @Resource109) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource108{
                    return <- create Resource108(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource109 {
                pub let c: UInt64 
                pub let r: @Resource110

                init(_ r: @Resource110) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource109{
                    return <- create Resource109(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource110 {
                pub let c: UInt64 
                pub let r: @Resource111

                init(_ r: @Resource111) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource110{
                    return <- create Resource110(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource111 {
                pub let c: UInt64 
                pub let r: @Resource112

                init(_ r: @Resource112) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource111{
                    return <- create Resource111(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource112 {
                pub let c: UInt64 
                pub let r: @Resource113

                init(_ r: @Resource113) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource112{
                    return <- create Resource112(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource113 {
                pub let c: UInt64 
                pub let r: @Resource114

                init(_ r: @Resource114) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource113{
                    return <- create Resource113(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource114 {
                pub let c: UInt64 
                pub let r: @Resource115

                init(_ r: @Resource115) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource114{
                    return <- create Resource114(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource115 {
                pub let c: UInt64 
                pub let r: @Resource116

                init(_ r: @Resource116) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource115{
                    return <- create Resource115(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource116 {
                pub let c: UInt64 
                pub let r: @Resource117

                init(_ r: @Resource117) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource116{
                    return <- create Resource116(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource117 {
                pub let c: UInt64 
                pub let r: @Resource118

                init(_ r: @Resource118) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource117{
                    return <- create Resource117(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource118 {
                pub let c: UInt64 
                pub let r: @Resource119

                init(_ r: @Resource119) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource118{
                    return <- create Resource118(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource119 {
                pub let c: UInt64 
                pub let r: @Resource120

                init(_ r: @Resource120) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource119{
                    return <- create Resource119(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource120 {
                pub let c: UInt64 
                pub let r: @Resource121

                init(_ r: @Resource121) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource120{
                    return <- create Resource120(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource121 {
                pub let c: UInt64 
                pub let r: @Resource122

                init(_ r: @Resource122) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource121{
                    return <- create Resource121(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource122 {
                pub let c: UInt64 
                pub let r: @Resource123

                init(_ r: @Resource123) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource122{
                    return <- create Resource122(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource123 {
                pub let c: UInt64 
                pub let r: @Resource124

                init(_ r: @Resource124) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource123{
                    return <- create Resource123(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource124 {
                pub let c: UInt64 
                pub let r: @Resource125

                init(_ r: @Resource125) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource124{
                    return <- create Resource124(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource125 {
                pub let c: UInt64 
                pub let r: @Resource126

                init(_ r: @Resource126) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource125{
                    return <- create Resource125(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource126 {
                pub let c: UInt64 
                pub let r: @Resource127

                init(_ r: @Resource127) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource126{
                    return <- create Resource126(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource127 {
                pub let c: UInt64 
                pub let r: @Resource128

                init(_ r: @Resource128) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource127{
                    return <- create Resource127(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource128 {
                pub let c: UInt64 
                pub let r: @Resource129

                init(_ r: @Resource129) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource128{
                    return <- create Resource128(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource129 {
                pub let c: UInt64 
                pub let r: @Resource130

                init(_ r: @Resource130) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource129{
                    return <- create Resource129(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource130 {
                pub let c: UInt64 
                pub let r: @Resource131

                init(_ r: @Resource131) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource130{
                    return <- create Resource130(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource131 {
                pub let c: UInt64 
                pub let r: @Resource132

                init(_ r: @Resource132) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource131{
                    return <- create Resource131(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource132 {
                pub let c: UInt64 
                pub let r: @Resource133

                init(_ r: @Resource133) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource132{
                    return <- create Resource132(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource133 {
                pub let c: UInt64 
                pub let r: @Resource134

                init(_ r: @Resource134) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource133{
                    return <- create Resource133(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource134 {
                pub let c: UInt64 
                pub let r: @Resource135

                init(_ r: @Resource135) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource134{
                    return <- create Resource134(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource135 {
                pub let c: UInt64 
                pub let r: @Resource136

                init(_ r: @Resource136) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource135{
                    return <- create Resource135(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource136 {
                pub let c: UInt64 
                pub let r: @Resource137

                init(_ r: @Resource137) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource136{
                    return <- create Resource136(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource137 {
                pub let c: UInt64 
                pub let r: @Resource138

                init(_ r: @Resource138) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource137{
                    return <- create Resource137(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource138 {
                pub let c: UInt64 
                pub let r: @Resource139

                init(_ r: @Resource139) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource138{
                    return <- create Resource138(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource139 {
                pub let c: UInt64 
                pub let r: @Resource140

                init(_ r: @Resource140) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource139{
                    return <- create Resource139(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource140 {
                pub let c: UInt64 
                pub let r: @Resource141

                init(_ r: @Resource141) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource140{
                    return <- create Resource140(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource141 {
                pub let c: UInt64 
                pub let r: @Resource142

                init(_ r: @Resource142) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource141{
                    return <- create Resource141(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource142 {
                pub let c: UInt64 
                pub let r: @Resource143

                init(_ r: @Resource143) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource142{
                    return <- create Resource142(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource143 {
                pub let c: UInt64 
                pub let r: @Resource144

                init(_ r: @Resource144) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource143{
                    return <- create Resource143(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource144 {
                pub let c: UInt64 
                pub let r: @Resource145

                init(_ r: @Resource145) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource144{
                    return <- create Resource144(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource145 {
                pub let c: UInt64 
                pub let r: @Resource146

                init(_ r: @Resource146) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource145{
                    return <- create Resource145(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource146 {
                pub let c: UInt64 
                pub let r: @Resource147

                init(_ r: @Resource147) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource146{
                    return <- create Resource146(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource147 {
                pub let c: UInt64 
                pub let r: @Resource148

                init(_ r: @Resource148) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource147{
                    return <- create Resource147(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource148 {
                pub let c: UInt64 
                pub let r: @Resource149

                init(_ r: @Resource149) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource148{
                    return <- create Resource148(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource149 {
                pub let c: UInt64 
                pub let r: @Resource150

                init(_ r: @Resource150) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource149{
                    return <- create Resource149(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource150 {
                pub let c: UInt64 
                pub let r: @Resource151

                init(_ r: @Resource151) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource150{
                    return <- create Resource150(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource151 {
                pub let c: UInt64 
                pub let r: @Resource152

                init(_ r: @Resource152) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource151{
                    return <- create Resource151(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource152 {
                pub let c: UInt64 
                pub let r: @Resource153

                init(_ r: @Resource153) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource152{
                    return <- create Resource152(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource153 {
                pub let c: UInt64 
                pub let r: @Resource154

                init(_ r: @Resource154) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource153{
                    return <- create Resource153(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource154 {
                pub let c: UInt64 
                pub let r: @Resource155

                init(_ r: @Resource155) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource154{
                    return <- create Resource154(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource155 {
                pub let c: UInt64 
                pub let r: @Resource156

                init(_ r: @Resource156) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource155{
                    return <- create Resource155(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource156 {
                pub let c: UInt64 
                pub let r: @Resource157

                init(_ r: @Resource157) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource156{
                    return <- create Resource156(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource157 {
                pub let c: UInt64 
                pub let r: @Resource158

                init(_ r: @Resource158) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource157{
                    return <- create Resource157(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource158 {
                pub let c: UInt64 
                pub let r: @Resource159

                init(_ r: @Resource159) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource158{
                    return <- create Resource158(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource159 {
                pub let c: UInt64 
                pub let r: @Resource160

                init(_ r: @Resource160) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource159{
                    return <- create Resource159(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource160 {
                pub let c: UInt64 
                pub let r: @Resource161

                init(_ r: @Resource161) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource160{
                    return <- create Resource160(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource161 {
                pub let c: UInt64 
                pub let r: @Resource162

                init(_ r: @Resource162) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource161{
                    return <- create Resource161(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource162 {
                pub let c: UInt64 
                pub let r: @Resource163

                init(_ r: @Resource163) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource162{
                    return <- create Resource162(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource163 {
                pub let c: UInt64 
                pub let r: @Resource164

                init(_ r: @Resource164) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource163{
                    return <- create Resource163(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource164 {
                pub let c: UInt64 
                pub let r: @Resource165

                init(_ r: @Resource165) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource164{
                    return <- create Resource164(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource165 {
                pub let c: UInt64 
                pub let r: @Resource166

                init(_ r: @Resource166) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource165{
                    return <- create Resource165(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource166 {
                pub let c: UInt64 
                pub let r: @Resource167

                init(_ r: @Resource167) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource166{
                    return <- create Resource166(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource167 {
                pub let c: UInt64 
                pub let r: @Resource168

                init(_ r: @Resource168) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource167{
                    return <- create Resource167(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource168 {
                pub let c: UInt64 
                pub let r: @Resource169

                init(_ r: @Resource169) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource168{
                    return <- create Resource168(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource169 {
                pub let c: UInt64 
                pub let r: @Resource170

                init(_ r: @Resource170) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource169{
                    return <- create Resource169(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource170 {
                pub let c: UInt64 
                pub let r: @Resource171

                init(_ r: @Resource171) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource170{
                    return <- create Resource170(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource171 {
                pub let c: UInt64 
                pub let r: @Resource172

                init(_ r: @Resource172) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource171{
                    return <- create Resource171(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource172 {
                pub let c: UInt64 
                pub let r: @Resource173

                init(_ r: @Resource173) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource172{
                    return <- create Resource172(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource173 {
                pub let c: UInt64 
                pub let r: @Resource174

                init(_ r: @Resource174) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource173{
                    return <- create Resource173(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource174 {
                pub let c: UInt64 
                pub let r: @Resource175

                init(_ r: @Resource175) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource174{
                    return <- create Resource174(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource175 {
                pub let c: UInt64 
                pub let r: @Resource176

                init(_ r: @Resource176) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource175{
                    return <- create Resource175(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource176 {
                pub let c: UInt64 
                pub let r: @Resource177

                init(_ r: @Resource177) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource176{
                    return <- create Resource176(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource177 {
                pub let c: UInt64 
                pub let r: @Resource178

                init(_ r: @Resource178) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource177{
                    return <- create Resource177(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource178 {
                pub let c: UInt64 
                pub let r: @Resource179

                init(_ r: @Resource179) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource178{
                    return <- create Resource178(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource179 {
                pub let c: UInt64 
                pub let r: @Resource180

                init(_ r: @Resource180) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource179{
                    return <- create Resource179(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource180 {
                pub let c: UInt64 
                pub let r: @Resource181

                init(_ r: @Resource181) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource180{
                    return <- create Resource180(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource181 {
                pub let c: UInt64 
                pub let r: @Resource182

                init(_ r: @Resource182) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource181{
                    return <- create Resource181(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource182 {
                pub let c: UInt64 
                pub let r: @Resource183

                init(_ r: @Resource183) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource182{
                    return <- create Resource182(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource183 {
                pub let c: UInt64 
                pub let r: @Resource184

                init(_ r: @Resource184) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource183{
                    return <- create Resource183(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource184 {
                pub let c: UInt64 
                pub let r: @Resource185

                init(_ r: @Resource185) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource184{
                    return <- create Resource184(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource185 {
                pub let c: UInt64 
                pub let r: @Resource186

                init(_ r: @Resource186) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource185{
                    return <- create Resource185(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource186 {
                pub let c: UInt64 
                pub let r: @Resource187

                init(_ r: @Resource187) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource186{
                    return <- create Resource186(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource187 {
                pub let c: UInt64 
                pub let r: @Resource188

                init(_ r: @Resource188) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource187{
                    return <- create Resource187(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource188 {
                pub let c: UInt64 
                pub let r: @Resource189

                init(_ r: @Resource189) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource188{
                    return <- create Resource188(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource189 {
                pub let c: UInt64 
                pub let r: @Resource190

                init(_ r: @Resource190) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource189{
                    return <- create Resource189(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource190 {
                pub let c: UInt64 
                pub let r: @Resource191

                init(_ r: @Resource191) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource190{
                    return <- create Resource190(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource191 {
                pub let c: UInt64 
                pub let r: @Resource192

                init(_ r: @Resource192) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource191{
                    return <- create Resource191(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource192 {
                pub let c: UInt64 
                pub let r: @Resource193

                init(_ r: @Resource193) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource192{
                    return <- create Resource192(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource193 {
                pub let c: UInt64 
                pub let r: @Resource194

                init(_ r: @Resource194) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource193{
                    return <- create Resource193(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource194 {
                pub let c: UInt64 
                pub let r: @Resource195

                init(_ r: @Resource195) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource194{
                    return <- create Resource194(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource195 {
                pub let c: UInt64 
                pub let r: @Resource196

                init(_ r: @Resource196) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource195{
                    return <- create Resource195(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource196 {
                pub let c: UInt64 
                pub let r: @Resource197

                init(_ r: @Resource197) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource196{
                    return <- create Resource196(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource197 {
                pub let c: UInt64 
                pub let r: @Resource198

                init(_ r: @Resource198) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource197{
                    return <- create Resource197(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource198 {
                pub let c: UInt64 
                pub let r: @Resource199

                init(_ r: @Resource199) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource198{
                    return <- create Resource198(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource199 {
                pub let c: UInt64 
                pub let r: @Resource200

                init(_ r: @Resource200) {
                    self.c = UInt64(1.0)
                    self.r <- r
                }
                pub fun duplicate(): @Resource199{
                    return <- create Resource199(<-self.r.duplicate())
                }

                pub fun check(): Bool {
                    let r <- create self.r.duplicate()
                    let c = r.check()
                    destroy r 
                    return c
                }

                destroy(){
                    pre {
                        self.check()
                    }
                    destroy self.r
                }
            }
            pub resource Resource200 {
                pub let c: UInt64 

                init() {
                    self.c = UInt64(1.0)
                }
                pub fun duplicate(): @Resource200{
                    return <- create Resource200()
                }

                pub fun check(): Bool {
                    return true
                }

                destroy(){
                    pre {
                        self.check()
                    }
                }
            }}

`
