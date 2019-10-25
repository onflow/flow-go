module FPL-TYPE-ANNOTATION
  syntax Type
  syntax Annot ::= Type | noAnnot()

endmodule

module FPL-TYPE-ANNOTATION-PROGRAM-PARSING
  syntax Type
  syntax Annot ::= Type
endmodule

module FPL-COMMON-SYNTAX
  /* This module defines most of the Flow AST,
     containing all productions whose actual syntax can be
     used in K rules without ambiguity.

     The remaining constructs are given their actual syntax for
     parsing programs in FPL-SYNTAX and a syntax that can be used
     in semantic rules in FPL-RULE-SYNTAX
   */
  imports FPL-TYPE-ANNOTATION
  imports UNSIGNED-INT-SYNTAX
  imports BOOL-SYNTAX

  syntax IntLit ::= Int
  syntax Ident
  syntax FPLString

  syntax Definition ::=
     FunDef
   | VarDef
   | "struct" Ident ":" TypeIds "{" CompositeItems "}"
   | "struct" Ident             "{" CompositeItems "}"
   | "resource" Ident ":" TypeIds "{" CompositeItems "}"
   | "resource" Ident             "{" CompositeItems "}"
   | "interface" Ident "{" InterfaceItems "}"
   // should there be separate "struct interface" and "resource interface"?
   | "impl" TypeId "for" TypeId "{" ImplBody "}"
//   | "using" TypeId "from" AddressLiteral
  syntax TopDefinition ::=
     Definition
   | "import" ImportSource
   | "import" TypeIds "from" ImportSource

  rule struct Ident { Items } => struct Ident : .TypeIds { Items } [macro]
  rule resource Ident { Items } => resource Ident : .TypeIds { Items } [macro]

  syntax Transaction ::=
     "transaction" "{"
        Fields
        "prepare" "(" Formals ")" "{" Block "}"
        "execute" "{" Block "}"
        "post" "{" Conditions "}"
     "}"
  syntax Fields ::= List{FieldDecl,""}

  syntax FileName ::= FPLString
  syntax ImportSource ::= IntLit | FPLString

  syntax FunDef ::= "fun" Ident "(" Formals ")"           FunBody
                  | "fun" Ident "(" Formals ")" ":" Annot FunBody

  rule fun Ident ( Formals ) Body::FunBody => fun Ident ( Formals ) : noAnnot() Body [macro]

  syntax CompositeItems ::= List{CompositeItemAccess,""}
  syntax CompositeItemAccess ::= Access CompositeItem
                               | CompositeItem
  syntax CompositeItem ::= Definition
                         | FieldDecl
                         | "init" "(" Formals ")" FunBody
  syntax Access ::= "pub" "(" "set" ")"
                  | "pub"

  syntax InterfaceItems ::= List{InterfaceItemAccess,""}
  syntax InterfaceItemAccess ::= Access InterfaceItem
                               | InterfaceItem
  syntax InterfaceItem ::= Definition
                         | InterfaceFieldDecl
                         | InterfaceFunDecl
                         | "init" "(" Formals ")"
  syntax InterfaceFieldDecl ::= "var" Ident ":" Type
                              | "var" Ident ":" Type "{" AccessorContracts "}"
                              | "let" Ident ":" Type
                              | "let" Ident ":" Type "{" AccessorContracts "}"
                              |       Ident ":" Type
                              |       Ident ":" Type "{" AccessorContracts "}"

  rule (var  Ident : Type)::InterfaceFieldDecl => var Ident : Type { .AccessorContracts } [macro]
  rule (let  Ident : Type)::InterfaceFieldDecl => let Ident : Type { .AccessorContracts } [macro]
  rule      (Ident : Type)::InterfaceFieldDecl =>     Ident : Type { .AccessorContracts } [macro]

  syntax InterfaceFunDecl ::= "fun" Ident "(" Formals ")"
                            | "fun" Ident "(" Formals ")"           ContractBlock
                            | "fun" Ident "(" Formals ")" ":" Annot
                            | "fun" Ident "(" Formals ")" ":" Annot ContractBlock

  rule fun Ident ( Formals )                      => fun Ident ( Formals ) : noAnnot() {} [macro]
  rule fun Ident ( Formals ) Block::ContractBlock => fun Ident ( Formals ) : noAnnot() Block [macro]
  rule fun Ident ( Formals ) : Annot              => fun Ident ( Formals ) : Annot {} [macro]

  syntax AccessorContracts ::= List{AccessorContract,""}
  syntax AccessorContract ::=
     "get"               ContractBlock
   | "set" "(" Ident ")" ContractBlock

  syntax ImplBody ::= List{ImplItemAccess,";"}
  syntax ImplItemAccess ::= Access ImplItem
                          | ImplItem
  syntax ImplItem ::= Definition
                    | FunDef
                    | FieldDecl // must be a synthetic field

  syntax FieldDecl ::=
     "let" Ident ":" Type
   | "var" Ident ":" Type
   | "synthetic" Ident "{" AccessorDefs "}"
  syntax AccessorDefs ::= List{AccessorDef,""}
  syntax AccessorDef ::= "get"               FunBody
                       | "set" "(" Ident ")" FunBody
  // Nesting left out for now

  syntax VarDef ::=
     "let" Ident            "=" Exp
   | "let" Ident ":" Annot  "=" Exp
   | "let" Ident           "<-" Exp
   | "let" Ident ":" Annot "<-" Exp
   | "var" Ident            "=" Exp
   | "var" Ident ":" Annot  "=" Exp
   | "var" Ident           "<-" Exp
   | "var" Ident ":" Annot "<-" Exp

  rule  let Ident  = Exp          => let Ident : noAnnot()  = Exp [macro]
  rule  let Ident <- Exp          => let Ident : noAnnot() <- Exp [macro]
  rule (var Ident  = Exp)::VarDef => var Ident : noAnnot()  = Exp [macro]
  rule  var Ident <- Exp          => var Ident : noAnnot() <- Exp [macro]

  syntax Formal ::= Ident Ident ":" Type
                  |       Ident ":" Type
  rule (I : T)::Formal => I I : T [macro]
  syntax Formals ::= List{Formal,","} [klabel(formals)]

  syntax FunBody ::= "{" Block "}"
                   | "{" Contract Block "}"
  syntax Contract ::= "pre" "{" Conditions "}"
                    | "post" "{" Conditions "}"
                    | "pre" "{" Conditions "}" "post" "{" Conditions "}"
  syntax ContractBlock ::= "{" Contract "}"
                         | "{" "}"

  rule { B } ::FunBody => { pre { .Conditions } post { .Conditions } B } ::FunBody [macro]

  rule {} => { pre { .Conditions } post { .Conditions } } [macro]
  rule pre { Cond } => pre { Cond } post { .Conditions } [macro]
  rule post { Cond } => pre { .Conditions } post { Cond } [macro]

  syntax Conditions ::= NeList{Condition,""}
  syntax Condition ::= Exp
                     | Exp ":" FPLString

  syntax Stmt ::= VarDef
                | FunDef
                | Path "=" Exp
                | Path "<-" Exp
                | Exp
                // Deploy?
                // Publish?
                | IfStmt
                | "while" Exp "{" Block "}"
                | "continue"
                | "break"
                | "return"
                | "return" Exp [strict]

  syntax IfStmt ::= "if" IfCond "{" Block "}"
                  | "if" IfCond "{" Block "}" "else" "{" Block "}"
                  | "if" IfCond "{" Block "}" "else" IfStmt

  rule if Cond { Block } => if Cond { Block } else { .Block } [macro]
  rule if Cond { Block } else If => if Cond { Block } else { If } [macro]

  syntax IfCond ::= Exp
                  | "var" Ident "=" Exp
  syntax Block ::= List{Stmt,";"}

  syntax Path ::= Ident
                | "self"
                | Path "[" Exp "]"
                | Path "." Ident

  syntax left lval

  syntax Exp ::= IntLit
               | FPLString
               | Ident
               | "self"
               | "nil"
               | Bool
               | "(" Exp ")" [bracket]
               | "[" Exps "]"        [strict(1)]
               | "[" Type "]"               // type-indexed collections
               | "{" DictEntries "}" [strict(1)]
               | "fun" "(" Formals ")" FunBody
               | "fun" "(" Formals ")" ":" Annot FunBody
               | Exp "[" Exp "]" [lval, seqstrict]
               | Exp "." Ident    [lval, strict(1)] // field access
               | Ident "(" Actuals ")" [strict(2), klabel(call)]
               | Exp "." Ident "(" Actuals ")" [seqstrict(1,3)]
               | TypeId "(" Actuals ")" [klabel(call)]          // struct creation
               | "create" TypeId "(" Actuals ")" // resource creation
               > non-assoc:
                 "-" Exp       [strict]
               | "!" Exp       [strict]
               > left:
                 Exp "*" Exp   [seqstrict]
               | Exp "&*" Exp  [seqstrict]
               | Exp "/" Exp   [seqstrict]
               | Exp "&/" Exp  [seqstrict]
               | Exp "%" Exp   [seqstrict]
               > left: Exp "+" Exp   [seqstrict]
               | Exp "&+" Exp  [seqstrict]
               | Exp "-" Exp   [seqstrict]
               | Exp "&-" Exp  [seqstrict]
               > left:
                 Exp "?" "?" Exp  [strict(1)]
               | Exp "as?" Exp [seqstrict]
               > left:
                 Exp "==" Exp  [seqstrict]
               | Exp "!=" Exp  [seqstrict]
               | Exp "<" Exp   [seqstrict]
               | Exp "<=" Exp  [seqstrict]
               | Exp ">" Exp   [seqstrict]
               | Exp ">=" Exp  [seqstrict]
               > left:
                 Exp "&&" Exp  [strict(1)]
               | Exp "||" Exp  [strict(1)]
               > Exp "?" Exp ":" Exp [right,strict(1)]
  syntax Exps ::= List{Exp,","}      [strict]

  rule fun ( Formals ) { Body } => fun ( Formals ) : noAnnot() { Body } [macro]

  syntax DictEntries ::= List{DictEntry,","} [seqstrict]
  syntax DictEntry ::= Exp ":" Exp [seqstrict]

  syntax KResult
  syntax Actual ::= Ident ":" Exp [hybrid, strict(2)]
                  | Exp
  syntax Actuals ::= List{Actual,","} [seqstrict, klabel(actuals), hybrid]
  rule isKResult(.Actuals) => true

  syntax TypeId ::= Ident // plus fully qualified names
  syntax TypeIds ::= List{TypeId,","}

  syntax Type ::= Ident
                | "Self"
                | "(" "(" Types ")" ":" Type ")"
                | Type "?"
                | Type "[" "]"
                | Type "[" IntLit "]"
                | Type "[" Type "]"
                > "@" Type
  syntax Types ::= List{Type,","}

  syntax Pgm ::= List{TopDefinition,";"}
endmodule

module FPL-TOKENS
  /* This modules defines the lexical syntax of
     tokens that we need for parsing programs, but
     but must not be parsed in K rules, to
     avoid ambiguity with K syntax
   */
  imports FPL-COMMON-SYNTAX
  syntax IntLit ::=
     r"-?0x[0-9a-fA-F]([_0-9a-fA-F]*[0-9a-fA-F])?" [token]
   | r"-?0o[0-7]([_0-7]*[0-7])?" [token]
   | r"-?0b[01]([_01]*[01])?" [token]
   | r"-?[0-9]+_[_0-9]*[0-9]" [token]
  syntax Ident ::=
     r"[_a-zA-Z][_a-zA-Z0-9]*" [token,autoReject]
  syntax FPLString ::=
     r"[\\\"](([^\"\\n\\r\\\\])|([\\\\][0\\\\tnr\"'])|([\\\\]u\\{[0-9a-fA-F]+\\}))*[\\\"]" [token]
endmodule

module FPL-SYNTAX
  /* This module defines the concrete syntax available in programs */
  imports FPL-COMMON-SYNTAX
  imports FPL-TOKENS
endmodule

module FPL-SCOPES
    syntax Ident
    syntax Scope ::= "global" | local(Ident)
endmodule

module FPL-VALUES
  imports BOOL-SYNTAX
  imports INT-SYNTAX
  imports STRING-SYNTAX

  syntax Type
  syntax Val ::= Int | Bool
  syntax TypedVal ::= v(Val, Type)
  syntax Exp ::= TypedVal
  syntax KResult ::= TypedVal | Int // for constant expressions
  syntax FPLString ::= String

endmodule

module FPL-ENV
  syntax TypedVal
  syntax Mutability ::= "let" | "var"
  syntax EnvEntry ::= env(mut: Mutability, v: TypedVal)
endmodule

module FPL-STACK
  imports DOMAINS
  imports FPL-ENV
  syntax Frame ::= frame(code:K, env: Map)
endmodule

module FPL-FUN-ENV
  imports FPL-COMMON-SYNTAX
  syntax FunEntry ::= funDef(args: Formals, result: Type, contract: Contract, body: Block)
endmodule

module FPL-PANIC
  imports FPL-CONFIGURATION

  syntax Exp ::= panic(String)

  rule <k> panic(_) ~> (_:KItem ~> _ => .K) </k>
endmodule

module FPL-TYPING
  imports FPL-CONFIGURATION
  imports FPL-ENV
  imports FPL-FUN-ENV
  imports FPL-PANIC

  syntax Type ::= type(Exp) [function]
                | #type(Exp) [function]
                | infer(Type) [function]
  syntax Ident ::= "Bool" [token]
                 | "Int8" [token]
                 | "Int16" [token]
                 | "Int32" [token]
                 | "Int64" [token]
                 | "UInt8" [token]
                 | "UInt16" [token]
                 | "UInt32" [token]
                 | "UInt64" [token]
                 | "Int" [token]

  syntax Ident ::= "IntLit" [token]

  rule type(E) => infer(#type(E))
  rule infer(IntLit) => Int
  rule infer(T) => T [owise]

  rule #type(I:Int) => IntLit
  rule #type(B:Bool) => Bool
  rule [[ #type(X:Ident) => T ]]
       <env>... X |-> env(... v: v(_, T)) ...</env>
  rule [[ #type(Fun:Ident(Provided:Actuals)) => R ]]
       <funs>... Fun |-> funDef(... args: Expected, result: R) ...</funs>
  rule #type(- E => E)
  rule #type(! _) => Bool
  rule #type(E1 * E2) => type(E1, E2)
  rule #type(E1 &* E2) => type(E1, E2)
  rule #type(E1 / E2) => type(E1, E2)
  rule #type(E1 % E2) => type(E1, E2)
  rule #type(E1 + E2) => type(E1, E2)
  rule #type(E1 &+ E2) => type(E1, E2)
  rule #type(E1 - E2) => type(E1, E2)
  rule #type(E1 &- E2) => type(E1, E2)
  rule #type(_ && _) => Bool
  rule #type(_ || _) => Bool
  rule #type(_ == _) => Bool
  rule #type(_ != _) => Bool
  rule #type(_ < _) => Bool
  rule #type(_ <= _) => Bool
  rule #type(_ > _) => Bool
  rule #type(_ >= _) => Bool
  rule #type(_ ? E1 : E2) => type(E1, E2)

  syntax Type ::= type(Exp, Exp) [function]
                | #type(Type, Type) [function]

  rule type(E1, E2) => #type(#type(E1), #type(E2))
  rule #type(IntLit, T) => T
  rule #type(T, IntLit) => T
  rule #type(T, T) => T

  syntax Int ::= "pow8" | "pow16" | "pow32" | "pow64"
  rule pow8 => 256 [macro]
  rule pow16 => 65536 [macro]
  rule pow32 => 4294967296 [macro]
  rule pow64 => 18446744073709551616 [macro]

  syntax Int ::= "min8" | "min16" | "min32" | "min64"
  rule min8 => -128 [macro]
  rule min16 => -32768 [macro]
  rule min32 => -2147483648 [macro]
  rule min64 => -9223372036854775808 [macro]

  syntax Exp ::= check(TypedVal) [function]
  rule check(v(V, T)) => #fun(V' => #if V' ==K v(V, T) #then v(V, T) #else panic("Integer overflow") #fi)(chop(v(V, T)))

  syntax Exp ::= chop(TypedVal) [function]
  rule chop(v(I, Int)) => v(I, Int)
  rule chop(v(I, UInt8)) => v(I modInt pow8, UInt8)
  rule chop(v(I, UInt16)) => v(I modInt pow16, UInt16)
  rule chop(v(I, UInt32)) => v(I modInt pow32, UInt32)
  rule chop(v(I, UInt64)) => v(I modInt pow64, UInt64)
  rule chop(v(I, Int8)) => v((I -Int min8) modInt pow8 +Int min8, Int8)
  rule chop(v(I, Int16)) => v((I -Int min16) modInt pow16 +Int min16, Int16)
  rule chop(v(I, Int32)) => v((I -Int min32) modInt pow32 +Int min32, Int32)
  rule chop(v(I, Int64)) => v((I -Int min64) modInt pow64 +Int min64, Int64)
endmodule

module FPL-CONFIGURATION
  imports DOMAINS
  imports FPL-COMMON-SYNTAX
  imports FPL-SCOPES
  imports FPL-VALUES
  imports FPL-STACK

  configuration
    <k>$PGM:Pgm</k>
    <scope> global </scope>
    <env> .Map </env>
    <stack> .List </stack>
    <funs> .Map </funs>
endmodule

module FPL-VARIABLE-DECLARATION
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  context let _ : _:Type = HOLE
    requires notBool isInt(HOLE)
  context var _ : _:Type = HOLE
    requires notBool isInt(HOLE)

  rule <k> let _ : T:Type = (V:Int => v(V, T)) ... </k>
  rule <k> var _ : T:Type = (V:Int => v(V, T)) ... </k>

  rule <k> let X : T:Type = v(V, T) => . ... </k>
       <env> Rho => Rho [ X <- env(let, v(V, T)) ] </env>
  rule <k> var X : T:Type = v(V, T) => . ... </k>
       <env> Rho => Rho [ X <- env(var, v(V, T)) ] </env>

  rule <k> let _ : (noAnnot() => type(E)) = E ... </k>
  rule <k> var _ : (noAnnot() => type(E)) = E ... </k>

  rule <k> X:Ident => V ... </k>
       <env>... X |-> env(... v: V) ...</env>
endmodule

module FPL-STMTS
  imports FPL-CONFIGURATION

  rule .Block => .K
  rule S; B::Block => S ~> B
endmodule

module FPL-FUNCTIONS
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  rule <k> fun F:Ident ( Fs ) : T { C B } => . ...</k>
       <funs> Rho => Rho [ F <- funDef(Fs, T, C, B) ] </funs>

  syntax Bool ::= checkLabels(Formals,Actuals) [function]
  rule checkLabels(.Formals, .Actuals) => true
  rule checkLabels(((Lbl _ : _), FS), ((ArgLbl : _), AS)) => checkLabels(FS,AS)
     when Lbl ==K ArgLbl
  rule checkLabels(((Lbl _ : _), FS), (_::Exp, AS)) => checkLabels(FS,AS)
     when Lbl ==K #token("_","Ident")
  rule checkLabels(_,_) => false [owise]

  syntax Map ::= initArgs(Formals,Actuals) [function]
               | initArg(Formal,Actual) [function]
  rule initArgs(.Formals, .Actuals) => .Map
  rule initArgs((F, FS), (A, AS)) => initArg(F,A) initArgs(FS,AS)

  rule initArg((Lbl Name : T), (ArgLbl : X)) =>
       Name |-> env(let, X)
     when Lbl ==K ArgLbl
  rule initArg((NoLbl Name : T), X:TypedVal) =>
       Name |-> env(let, X)
     when NoLbl ==K #token("_","Ident")

  rule <k>((F:Ident)(Args) ~> K) => B:Block</k>
       <funs>... F |-> funDef(... args: Params, body: B) ...</funs>
       <env>Env => initArgs(Params, Args) </env>
       <stack> .List => ListItem(frame(K, Env)) ...</stack>
  rule <k>return X:TypedVal ~> _ => X ~> K</k>
       <env>_ => Env</env>
       <stack> ListItem(frame(K,Env)) => .List ...</stack>

endmodule

module FPL-ARITHMETIC
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  syntax Exp ::= intExp(Exp, Exp)
  rule intExp(L, R)
    => ((L * R)
   #Or ((L &* R)
   #Or ((L / R)
   #Or ((L &/ R)
   #Or ((L % R)
   #Or ((L + R)
   #Or ((L &+ R)
   #Or ((L - R)
   #Or ((L &- R)
   #Or ((L == R)
   #Or ((L != R)
   #Or ((L < R)
   #Or ((L <= R)
   #Or ((L > R)
   #Or (L >= R)))))))))))))))::Exp [macro]

  rule <k> intExp(I1:Int, v(I2, T)) #as O::Exp => fillExp(v(I1, T), v(I2, T), O) ... </k>
  rule <k> intExp(v(I1, T), I2:Int) #as O::Exp => fillExp(v(I1, T), v(I2, T), O) ... </k>

  syntax Exp ::= fillExp(TypedVal, TypedVal, Exp) [function]
  rule fillExp(L, R, _ * _) => L * R
  rule fillExp(L, R, _ &* _) => L &* R
  rule fillExp(L, R, _ / _) => L / R
  rule fillExp(L, R, _ &/ _) => L &/ R
  rule fillExp(L, R, _ % _) => L % R
  rule fillExp(L, R, _ + _) => L + R
  rule fillExp(L, R, _ &+ _) => L &+ R
  rule fillExp(L, R, _ - _) => L - R
  rule fillExp(L, R, _ &- _) => L &- R
  rule fillExp(L, R, _ == _) => L == R
  rule fillExp(L, R, _ != _) => L != R
  rule fillExp(L, R, _ < _) => L < R
  rule fillExp(L, R, _ <= _) => L <= R
  rule fillExp(L, R, _ > _) => L > R
  rule fillExp(L, R, _ >= _) => L >= R

  rule <k> B:Bool => v(B, Bool) ... </k>

  rule <k> - I:Int => 0 -Int I ... </k>
  rule <k> - v(I, T) => v(0 -Int I, T) ... </k>

  rule <k> ! v(B, T) => v(notBool B, T) ... </k>

  rule <k> I1  * I2 => I1 *Int I2 ... </k>
  rule <k> I1 &* I2 => I1 *Int I2 ... </k>
  rule <k> I1  / I2 => I1 /Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1 &/ I2 => I1 /Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1  % I2 => I1 %Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1  + I2 => I1 +Int I2 ... </k>
  rule <k> I1 &+ I2 => I1 +Int I2 ... </k>
  rule <k> (I1  - I2)::Exp => I1 -Int I2 ... </k>
  rule <k> I1 &- I2 => I1 -Int I2 ... </k>
  rule <k> I1 == I2 => v(I1 ==Int I2, Bool) ... </k>
  rule <k> I1 != I2 => v(I1 =/=Int I2, Bool) ... </k>
  rule <k> I1 < I2 => v(I1 <Int I2, Bool) ... </k>
  rule <k> I1 <= I2 => v(I1 <=Int I2, Bool) ... </k>
  rule <k> I1 > I2 => v(I1 >Int I2, Bool) ... </k>
  rule <k> I1 >= I2 => v(I1 >=Int I2, Bool) ... </k>

  rule <k> v(I1, T)  * v(I2, T) => check(v(I1 *Int I2, T)) ... </k>
  rule <k> v(I1, T) &* v(I2, T) => chop (v(I1 *Int I2, T)) ... </k>
  rule <k> v(I1, T)  / v(I2, T) => check(v(I1 /Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T) &/ v(I2, T) => chop (v(I1 /Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T)  % v(I2, T) => check(v(I1 %Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T)  + v(I2, T) => check(v(I1 +Int I2, T)) ... </k>
  rule <k> v(I1, T) &+ v(I2, T) => chop (v(I1 +Int I2, T)) ... </k>
  rule <k> (v(I1, T)  - v(I2, T))::Exp => check(v(I1 -Int I2, T)) ... </k>
  rule <k> v(I1, T) &- v(I2, T) => chop (v(I1 -Int I2, T)) ... </k>
  rule <k> v(I1, _) == v(I2, _) => v(I1 ==Int I2, Bool) ... </k>
  rule <k> v(I1, _) != v(I2, _) => v(I1 =/=Int I2, Bool) ... </k>
  rule <k> v(I1, T) < v(I2, T) => v(I1 <Int I2, Bool) ... </k>
  rule <k> v(I1, T) <= v(I2, T) => v(I1 <=Int I2, Bool) ... </k>
  rule <k> v(I1, T) > v(I2, T) => v(I1 >Int I2, Bool) ... </k>
  rule <k> v(I1, T) >= v(I2, T) => v(I1 >=Int I2, Bool) ... </k>

  rule <k> v(B1, _) == v(B2, _) => v(B1 ==Bool B2, Bool) ... </k>
  rule <k> v(B1, _) != v(B2, _) => v(B1 =/=Bool B2, Bool) ... </k>

  rule <k> v(true, T) && E => E ... </k>
  rule <k> v(false, T) && _ => v(false, Bool) ... </k>
  rule <k> v(true, T) || E => v(true, Bool) ... </k>
  rule <k> v(false, T) || E => E ... </k>

  rule v(true, T) ? E1 : E2 => giveType(E1, type(E1, E2))
  rule v(false, T) ? E1 : E2 => giveType(E2, type(E1, E2))

  syntax Exp ::= giveType(Exp, Type) [strict(1)]
  rule giveType(v(...) #as V, _) => V
  rule giveType(I:Int, IntLit) => I
  rule giveType(I:Int, T) => v(I, T) requires T =/=K IntLit

endmodule

module FPL
  imports FPL-CONFIGURATION
  imports FPL-VARIABLE-DECLARATION
  imports FPL-STMTS
  imports FPL-FUNCTIONS
  imports FPL-ARITHMETIC

  rule T::TopDefinition ; Ts::Pgm => T ~> Ts
endmodule

module FPL-TESTING-COMMON-SYNTAX
  imports FPL-COMMON-SYNTAX
  syntax Definition ::= "assert" Exp [strict]
endmodule

module FPL-TESTING-SYNTAX
  imports FPL-SYNTAX
  imports FPL-TESTING-COMMON-SYNTAX
endmodule

module FPL-TESTING
  imports FPL
  imports FPL-TESTING-COMMON-SYNTAX

  rule <k> assert v(true,Bool) => . ... </k>

  rule <k> .Pgm #Or panic(_) </k>
       <env> E => .Map </env>
       <funs> _ => .Map </funs>
    requires E =/=K .Map

  rule <k> panic(S::String => "") </k>
    requires S =/=String ""
endmodule
