module BPL-LITERALS
  imports INT-SYNTAX
  syntax IntLit ::=
     r"-?0x[0-9a-fA-F]([_0-9a-fA-F]*[0-9a-fA-F])?" [prec(1),token]
   | r"-?0o[0-7]([_0-7]*[0-7])?" [prec(1),token]
   | r"-?0b[01]([_01]*[01])?" [prec(1),token]
   | Int [token]
   | r"-?[0-9]+_[_0-9]*[0-9]" [prec(1),token]
  syntax Id ::=
     r"(?<![A-Za-z0-9\\_])[_a-zA-Z][_a-zA-Z0-9]*" [prec(1), token, notInRules, autoReject]
     /*
  syntax BPLString ::= r"[\\\"](([^\\\"\\n\\r\\\\])|([\\\\][nrtf\\\"\\\\])|([\\\\][x][0-9a-fA-F]{2})|([\\\\][u][0-9a-fA-F]{4})|([\\\\][U][0-9a-fA-F]{8}))*[\\\"]"      [token]
    */
  syntax BPLString ::=
     r"[\\\"](([^\"\\n\\r\\\\])|([\\\\][0\\\\tnr\"'])|([\\\\]u\\{[0-9a-fA-F]+\\}))*[\\\"]" [token]
endmodule

module BPL-SYNTAX
  imports INT-SYNTAX
  imports STRING-SYNTAX
  imports BPL-LITERALS

  syntax Definition ::=
     FunDef
   | VarDef
   | "struct" Id OptConformances "{" CompositeItems "}"
   | "resource" Id OptConformances "{" CompositeItems "}"
   | "interface" Id "{" InterfaceItems "}"
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

  syntax AddressLiteral ::= r"0x[0-9a-fA-F]+" [token]
  syntax FileName ::= BPLString
  syntax ImportSource ::= IntLit | BPLString

  syntax FunDef ::= "fun" Id "(" Formals ")" OptAnnot "{" FunBody "}"

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
  syntax InterfaceFieldDecl ::= "var" Id ":" Type OptFieldContracts
                              | "let" Id ":" Type OptFieldContracts
                              |       Id ":" Type OptFieldContracts
  syntax OptFieldContracts ::= "{" AccessorContracts "}"
                             | ""
  syntax InterfaceFunDecl ::= "fun" Id "(" Formals ")" OptAnnot
                            | "fun" Id "(" Formals ")" OptAnnot "{" Contract "}"

  syntax AccessorContracts ::= List{AccessorContract,""}
  syntax AccessorContract ::=
     "get" "{" Contract "}"
   | "set" "(" Id ")" "{" Contract "}"

  syntax ImplBody ::= List{ImplItem,";"}
  syntax ImplItem ::= Access Definition
                    | Access FunDef
                    | Access FieldDecl // must be a synthetic field

  syntax FieldDecl ::=
     "let" Id ":" Type
   | "var" Id ":" Type
   | "synthetic" Id "{" AccessorDefs "}"
  syntax AccessorDefs ::= List{AccessorDef,""}
  syntax AccessorDef ::= "get" "{" FunBody "}"
                       | "set" "(" Id ")" "{" FunBody "}"
  // Nesting left out for now

  syntax VarDef ::=
     "let" Id OptAnnot "=" Exp
   | "let" Id OptAnnot "<-" Exp
   | "var" Id OptAnnot "=" Exp
   | "var" Id OptAnnot "<-" Exp

  syntax Formal ::= Id Id ":" Type
                  |    Id ":" Type
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
                  | "var" Id "=" Exp
  syntax Block ::= List{Stmt,";"}

  syntax Path ::= Id
                | "self"
                | Path "[" Exp "]"
                | Path "." Id

  syntax Exp ::= IntLit
               | BPLString
               | Id
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
               | Exp "." Id    [left, strict(1)] // field access
               | Id "(" Actuals ")" [strict(2)]
               | Exp "." Id "(" Actuals ")" [seqstrict(1,3)]
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

  syntax Actual ::= Id ":" Exp [strict(2)]
                  | Exp
  syntax Actuals ::= List{Actual,","}

  syntax TypeId ::= Id // plus fully qualified names
  syntax TypeIds ::= List{TypeId,","}

  syntax Type ::= Id
                | "Self"
                | "(" "(" Types ")" ":" Type ")"
                | Type "?"
                | Type "[" "]"
                | Type "[" IntLit "]"
                | Type "[" Type "]"
                > "@" Type
  syntax Types ::= List{Type,","}

  syntax Pgm ::= List{TopDefinition,";"}

  syntax KResult ::= Int
endmodule

module BPL
  imports DOMAINS
  imports BPL-SYNTAX

  configuration
    <k>$PGM:Pgm</k>
endmodule
