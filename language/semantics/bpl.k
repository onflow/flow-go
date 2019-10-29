module BPL-TOKENS
  /* This module defines concrete token syntax that
     we want to allow in programs but not in rules
     of this semantics, to avoid ambiguity with K syntax.
   */
  imports BPL-TERMS
  syntax IntLit ::=
     r"-?0x[0-9a-fA-F]([_0-9a-fA-F]*[0-9a-fA-F])?" [token]
   | r"-?0o[0-7]([_0-7]*[0-7])?" [token]
   | r"-?0b[01]([_01]*[01])?" [token]
   | r"-?[0-9]+_[_0-9]*[0-9]" [token]
  syntax Ident ::=
     r"[_a-zA-Z][_a-zA-Z0-9]*" [token,autoReject]
  syntax BPLString ::=
     r"[\\\"](([^\"\\n\\r\\\\])|([\\\\][0\\\\tnr\"'])|([\\\\]u\\{[0-9a-fA-F]+\\}))*[\\\"]" [token]
endmodule

module BPL-TERMS
  imports INT-SYNTAX

  syntax IntLit ::= Int [token]
  syntax Ident
  syntax BPLString

  syntax Definition ::=
     FunDef
   | VarDef
   | "struct" Ident OptConformances "{" CompositeItems "}"
   | "resource" Ident OptConformances "{" CompositeItems "}"
   | "interface" Ident "{" InterfaceItems "}"
   // should there be separate "struct interface" and "resource interface"?
   | "impl" TypeId "for" TypeId "{" ImplBody "}"
//   | "using" TypeId "from" AddressLiteral
  syntax TopDefinition ::=
     Definition
   | "import" ImportSource
   | "import" TypeIds "from" ImportSource

  syntax Transaction ::=
     "transaction" "{"
        Fields
        "prepare" "(" Formals ")" "{" Block "}"
        "execute" "{" Block "}"
        "post" "{" Conditions "}"
     "}"
  syntax Fields ::= List{FieldDecl,""}

  syntax FileName ::= BPLString
  syntax ImportSource ::= IntLit | BPLString

  syntax FunDef ::= "fun" Ident "(" Formals ")" OptAnnot "{" FunBody "}"

  syntax CompositeItems ::= List{CompositeItem,""}
  syntax CompositeItem ::= Access Definition
                         | Access FieldDecl
                         | Access "init" "(" Formals ")" "{" FunBody "}"
  syntax Access ::= "pub" "(" "set" ")"
                  | "pub"
                  | ""

  syntax OptConformances ::= "" | ":" TypeIds

  syntax InterfaceItems ::= List{InterfaceItem,""}
  syntax InterfaceItem ::= Access Definition
                         | Access InterfaceFieldDecl
                         | Access InterfaceFunDecl
                         | Access "init" "(" Formals ")"
  syntax InterfaceFieldDecl ::= "var" Ident ":" Type OptFieldContracts
                              | "let" Ident ":" Type OptFieldContracts
                              |       Ident ":" Type OptFieldContracts
  syntax OptFieldContracts ::= "{" AccessorContracts "}"
                             | ""
  syntax InterfaceFunDecl ::= "fun" Ident "(" Formals ")" OptAnnot
                            | "fun" Ident "(" Formals ")" OptAnnot "{" Contract "}"

  syntax AccessorContracts ::= List{AccessorContract,""}
  syntax AccessorContract ::=
     "get" "{" Contract "}"
   | "set" "(" Ident ")" "{" Contract "}"

  syntax ImplBody ::= List{ImplItem,";"}
  syntax ImplItem ::= Access Definition
                    | Access FunDef
                    | Access FieldDecl // must be a synthetic field

  syntax FieldDecl ::=
     "let" Ident ":" Type
   | "var" Ident ":" Type
   | "synthetic" Ident "{" AccessorDefs "}"
  syntax AccessorDefs ::= List{AccessorDef,""}
  syntax AccessorDef ::= "get" "{" FunBody "}"
                       | "set" "(" Ident ")" "{" FunBody "}"
  // Nesting left out for now

  syntax VarDef ::=
     "let" Ident OptAnnot "=" Exp
   | "let" Ident OptAnnot "<-" Exp
   | "var" Ident OptAnnot "=" Exp
   | "var" Ident OptAnnot "<-" Exp

  syntax Formal ::= Ident Ident ":" Type
                  |       Ident ":" Type
  syntax Formals ::= List{Formal,","}

  syntax FunBody ::= Contract Block
  syntax Contract ::= OptPre OptPost
  syntax OptPre ::= "pre" "{" Conditions "}"
                  | ""
  syntax OptPost ::= "post" "{" Conditions "}"
                    | ""
  syntax Conditions ::= NeList{Condition,""}
  syntax Condition ::= Exp OptCondMessage
  syntax OptCondMessage ::= ":" BPLString
                          | ""

  syntax OptAnnot ::= ":" Type
                    | ""    [klabel(noAnnot)]

  syntax Stmt ::= VarDef
                | FunDef
                | Path "=" Exp
                | Path "<-" Exp
                | Exp
                // Deploy?
                // Publish?
                | "if" IfCond "{" Block "}" Elses
                | "while" Exp "{" Block "}"
                | "continue"
                | "break"
                | "return"
                | "return" Exp
  syntax Elses ::= ""
                 | "else" "{" Block "}"
                 | "else" "if" IfCond "{" Block "}" Elses
  syntax IfCond ::= Exp
                  | "var" Ident "=" Exp
  syntax Block ::= List{Stmt,";"}

  syntax Path ::= Ident
                | "self"
                | Path "[" Exp "]"
                | Path "." Ident

  syntax Exp ::= IntLit
               | BPLString
               | Ident
               | "self"
               | "nil"
               | "true"
               | "false"
               | "(" Exp ")" [bracket]
               | "[" Exps "]"        [strict(1)]
               | "[" Type "]"               // type-indexed collections
               | "{" DictEntries "}" [strict(1)]
               | "fun" "(" Formals ")" OptAnnot "{" FunBody "}"
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

module BPL-SYNTAX
  imports BPL-TERMS
  imports BPL-TOKENS
endmodule

module BPL
  imports DOMAINS
  imports BPL-TERMS

  syntax KResult ::= Int
  configuration
    <k>$PGM:Pgm</k>
endmodule
