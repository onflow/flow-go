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

  syntax FunDef ::= "fun" Ident "(" Formals ")"           "{" FunBody "}"
                  | "fun" Ident "(" Formals ")" ":" Annot "{" FunBody "}"

  rule fun Ident ( Formals ) { Body::FunBody } => fun Ident ( Formals ) : noAnnot() { Body } [macro]

  syntax CompositeItems ::= List{CompositeItemAccess,""}
  syntax CompositeItemAccess ::= Access CompositeItem
                               | CompositeItem
  syntax CompositeItem ::= Definition
                         | FieldDecl
                         | "init" "(" Formals ")" "{" FunBody "}"
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
  syntax AccessorDef ::= "get"               "{" FunBody "}"
                       | "set" "(" Ident ")" "{" FunBody "}"
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
  syntax Formals ::= List{Formal,","}

  syntax FunBody ::= Block
                   | Contract Block
  syntax Contract ::= "pre" "{" Conditions "}"
                    | "post" "{" Conditions "}"
                    | "pre" "{" Conditions "}" "post" "{" Conditions "}"
  syntax ContractBlock ::= "{" Contract "}"
                         | "{" "}"

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
                | "return" Exp

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
               | "fun" "(" Formals ")" "{" FunBody "}"
               | "fun" "(" Formals ")" ":" Annot "{" FunBody "}"
               | Exp "[" Exp "]" [left, seqstrict]
               | Exp "." Ident    [left, strict(1)] // field access
               | Ident "(" Actuals ")" [strict(2)]
               | Exp "." Ident "(" Actuals ")" [seqstrict(1,3)]
               | TypeId "(" Actuals ")"          // struct creation
               | "create" TypeId "(" Actuals ")" // resource creation
               > "-" Exp       [      strict]
               | "!" Exp       [      strict]
               > Exp "*" Exp   [left, seqstrict]
               | Exp "&*" Exp  [left, seqstrict]
               | Exp "/" Exp   [left, seqstrict]
               | Exp "%" Exp   [left, seqstrict]
               > Exp "+" Exp   [left, seqstrict]
               | Exp "&+" Exp  [left, seqstrict]
               | Exp "-" Exp   [left, seqstrict]
               | Exp "&-" Exp  [left, seqstrict]
               > Exp "?" "?" Exp  [left, strict(1)]
               | Exp "as?" Exp [      seqstrict]
               > Exp "==" Exp  [      seqstrict]
               | Exp "!=" Exp  [      seqstrict]
               | Exp "<" Exp   [      seqstrict]
               | Exp "<=" Exp  [      seqstrict]
               | Exp ">" Exp   [      seqstrict]
               | Exp ">=" Exp  [      seqstrict]
               > Exp "&&" Exp  [left, seqstrict]
               | Exp "||" Exp  [left, seqstrict]
               > Exp "?" Exp ":" Exp [right,strict(1)]
  syntax Exps ::= List{Exp,","}      [strict]

  rule fun ( Formals ) { Body } => fun ( Formals ) : noAnnot() { Body } [macro]

  syntax DictEntries ::= List{DictEntry,","} [seqstrict]
  syntax DictEntry ::= Exp ":" Exp [seqstrict]

  syntax Actual ::= Ident ":" Exp [strict(2)]
                  | Exp
  syntax Actuals ::= List{Actual,","}

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

  syntax Val ::= Int | Bool
  syntax Exp ::= Val
  syntax KResult ::= Val

endmodule

module FPL-ENV
  syntax Val
  syntax Type
  syntax Mutability ::= "let" | "var"
  syntax EnvEntry ::= env(mut: Mutability, v: Val, t: Type)
endmodule

module FPL-TYPING
  imports FPL-CONFIGURATION
  imports FPL-ENV

  syntax Type ::= type(Exp) [function]
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

  rule type(I:Int) => Int
  rule type(B:Bool) => Bool
  rule [[ type(X:Ident) => T ]]
       <env>... X |-> env(... t: T) ...</env>
  rule type(- E => E)
  rule type(! _) => Bool
  rule type(_ * _) => Int
  rule type(E &* _ => E)
  rule type(_ / _) => Int
  rule type(_ % _) => Int
  rule type(_ + _) => Int
  rule type(E &+ _ => E)
  rule type(_ - _) => Int
  rule type(E &- _ => E)
  rule type(_ && _) => Bool
  rule type(_ || _) => Bool
  rule type(_ == _) => Bool
  rule type(_ != _) => Bool
  rule type(_ < _) => Bool
  rule type(_ <= _) => Bool
  rule type(_ > _) => Bool
  rule type(_ >= _) => Bool
  rule type(_ ? E : _) => E
  
endmodule

module FPL-CONFIGURATION
  imports DOMAINS
  imports FPL-COMMON-SYNTAX
  imports FPL-SCOPES
  imports FPL-VALUES

  configuration
    <k>$PGM:Pgm</k>
    <scope> global </scope>
    <env> .Map </env>
endmodule

module FPL-VARIABLE-DECLARATION
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  context let _ : _:Type = HOLE
  context var _ : _:Type = HOLE

  rule <k> let X : T:Type = V:Val => . ... </k>
       <env> Rho => Rho [ X <- env(let, V, T) ] </env>
  rule <k> var X : T:Type = V:Val => . ... </k>
       <env> Rho => Rho [ X <- env(var, V, T) ] </env>

  rule <k> let _ : (noAnnot() => type(E)) = E ... </k>
  rule <k> var _ : (noAnnot() => type(E)) = E ... </k>

  rule <k> X:Ident => V ... </k>
       <env>... X |-> env(... v: V) ...</env>
endmodule

module FPL-ARITHMETIC
  imports FPL-CONFIGURATION

  rule <k> I1 + I2 => I1 +Int I2 ... </k>
  rule <k> I1 == I2 => I1 ==Int I2 ... </k>
  rule <k> B1 == B2 => B1 ==Bool B2 ... </k>

endmodule

module FPL
  imports FPL-CONFIGURATION
  imports FPL-VARIABLE-DECLARATION
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

  rule <k> assert true => . ... </k>

  rule <k> .Pgm </k>
       <env> E => .Map </env>
    requires E =/=K .Map
endmodule
