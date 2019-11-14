module VERIFICATION
  imports FPL-TESTING
  syntax Ident ::= "sum" [token]
                 | "n" [token]
                 | "s" [token]
                 | "i" [token]
endmodule

module SUMPROOF
  imports FPL-TESTING
  imports VERIFICATION

rule
  <k>
    sum ( N , .Actuals ) ~> K => v((N *Int (N +Int 1)) /Int 2, Int) ~> K
  </k>
  <env>...
    sum |-> env ( let , ( ( Int , .Types ) : Int ) , LOC )
  ...</env>
  <store>...
    LOC |-> funDef ( _ n : Int , .Formals , Int , pre { .Conditions } post { .Conditions } , var s : noAnnot ( ) = 0 ; var i : noAnnot ( ) = 0 ; while i <= n { s = s + i ; i = i + 1 ; .Block } ; return s ; .Block , sum |-> env ( let , ( ( Int , .Types ) : Int ) , 0 ) )
  ...</store>

rule
  <k>
    while (i <= n) { s = s + i ; i = i + 1 } ~> K => K
  </k>
  <env>...
    i |-> env ( var , Int , 3 )
    n |-> env ( let , Int , 1 )
    s |-> env ( var , Int , 2 )
  ...</env>
  <store>...
    1 |-> N
    2 |-> (I => N)
    3 |-> (S => S +Int 3 *Int (N -Int I) *Int (N -Int I +Int 1) /Int 2)
  ...</store>
endmodule
