grammar Strictus;

// for handling optional semicolons between statement, see also `eos` rule

// NOTE: unusued builder variable, to avoid unused import error because
//    import will also be added to visitor code
@parser::header {
    import "strings"
    var _ = strings.Builder{}
}

@parser::members {
    // Returns true if on the current index of the parser's
    // token stream a token exists on the Hidden channel which
    // either is a line terminator, or is a multi line comment that
    // contains a line terminator.
    func (p *StrictusParser) lineTerminatorAhead() bool {
        // Get the token ahead of the current index.
        possibleIndexEosToken := p.GetCurrentToken().GetTokenIndex() - 1
        ahead := p.GetTokenStream().Get(possibleIndexEosToken)

        if ahead.GetChannel() != antlr.LexerHidden {
            // We're only interested in tokens on the HIDDEN channel.
            return true
        }

        if ahead.GetTokenType() == StrictusParserTerminator {
            // There is definitely a line terminator ahead.
            return true
        }

        if ahead.GetTokenType() == StrictusParserWS {
            // Get the token ahead of the current whitespaces.
            possibleIndexEosToken = p.GetCurrentToken().GetTokenIndex() - 2
            ahead = p.GetTokenStream().Get(possibleIndexEosToken)
        }

        // Get the token's text and type.
        text := ahead.GetText()
        _type := ahead.GetTokenType()

        // Check if the token is, or contains a line terminator.
        return (_type == StrictusParserBlockComment && (strings.Contains(text, "\r") || strings.Contains(text, "\n"))) ||
            (_type == StrictusParserTerminator)
    }
}

program
    : declaration* EOF
    ;

declaration
    : functionDeclaration
    | variableDeclaration
    ;

functionDeclaration
    : Pub? Fun Identifier '(' parameterList? ')' (':' returnType=typeName)? '{' block '}'
    ;

parameterList
    : parameter (',' parameter)*
    ;

parameter
    : Identifier ':' typeName
    ;

typeName
    : baseType typeDimension*
    ;

typeDimension
    : '[' DecimalLiteral? ']'
    ;

baseType
    : Identifier
    | '(' (parameterTypes+=typeName (',' parameterTypes+=typeName)*)? ')' '=>' returnType=typeName
    ;

block
    : (statement eos)*
    ;

statement
    : returnStatement
    | ifStatement
    | whileStatement
    | declaration
    | assignment
    | expression
    ;

returnStatement
    : Return expression?
    ;

ifStatement
    : If test=expression '{' then=block '}' (Else (ifStatement | '{' alt=block '}'))?
    ;

whileStatement
    : While expression '{' block '}'
    ;

variableDeclaration
    : (Const | Var) Identifier (':' typeName)? '=' expression
    ;

assignment
	: Identifier expressionAccess* '=' expression
	;

expression
    : conditionalExpression
    ;

conditionalExpression
	: <assoc=right> orExpression ('?' then=expression ':' alt=expression)?
	;

orExpression
	: andExpression ('||' andExpression)?
	;

andExpression
	: equalityExpression ('&&' equalityExpression)?
	;

equalityExpression
	: relationalExpression (equalityOp relationalExpression)?
	;

relationalExpression
	: additiveExpression (relationalOp additiveExpression)?
	;

additiveExpression
	: multiplicativeExpression (additiveOp multiplicativeExpression)?
	;

multiplicativeExpression
	: primaryExpression (multiplicativeOp primaryExpression)?
	;

primaryExpression
    : primaryExpressionStart primaryExpressionSuffix*
    ;

primaryExpressionSuffix
    : expressionAccess
    | invocation
    ;

equalityOp
    : Equal
    | Unequal
    ;

Equal : '==' ;
Unequal : '!=' ;

relationalOp
    : Less
    | Greater
    | LessEqual
    | GreaterEqual
    ;

Less : '<' ;
Greater : '>' ;
LessEqual : '<=' ;
GreaterEqual : '>=' ;

additiveOp
    : Plus
    | Minus
    ;

Plus : '+' ;
Minus : '-' ;

multiplicativeOp
    : Mul
    | Div
    | Mod
    ;

Mul : '*' ;
Div : '/' ;
Mod : '%' ;


primaryExpressionStart
    : Identifier                                                           # IdentifierExpression
    | literal                                                              # LiteralExpression
    | Fun '(' parameterList? ')' (':' returnType=typeName)? '{' block '}'  # FunctionExpression
    | '(' expression ')'                                                   # NestedExpression
    ;

expressionAccess
    : memberAccess
    | bracketExpression
    ;

memberAccess
	: '.' Identifier
	;

bracketExpression
	: '[' expression ']'
	;

invocation
	: '(' (expression (',' expression)*)? ')'
	;

literal
    : integerLiteral
    | booleanLiteral
    | arrayLiteral
    ;

booleanLiteral
    : True
    | False
    ;

integerLiteral
    : DecimalLiteral       # DecimalLiteral
    | BinaryLiteral        # BinaryLiteral
    | OctalLiteral         # OctalLiteral
    | HexadecimalLiteral   # HexadecimalLiteral
    ;

arrayLiteral
    : '[' ( expression (',' expression)* )? ']'
    ;

Fun : 'fun' ;

Pub : 'pub' ;

Return : 'return' ;
Const : 'const' ;
Var : 'var' ;

If : 'if' ;
Else : 'else' ;

While : 'while' ;

True : 'true' ;
False : 'false' ;

Identifier
    : IdentifierHead IdentifierCharacter*
    ;

fragment IdentifierHead
    : [a-zA-Z]
    |  '_'
    ;

fragment IdentifierCharacter
    : [0-9]
    | IdentifierHead
    ;

DecimalLiteral
    : [0-9] [0-9_]*
    ;
BinaryLiteral
    : '0b' [01]*
    ;

OctalLiteral
    : '0o' [0-7]*
    ;

HexadecimalLiteral
    : '0x' [0-9a-fA-F]*
    ;

WS
    : [ \t\u000B\u000C\u0000]+ -> channel(HIDDEN)
    ;

Terminator
	: [\r\n]+ -> channel(HIDDEN)
	;

BlockComment
    : '/*' (BlockComment|.)*? '*/'	-> channel(HIDDEN) // nesting comments allowed
    ;

LineComment
    : '//' ~[\r\n]* -> channel(HIDDEN)
    ;

eos
    : ';'
    | EOF
    | {p.lineTerminatorAhead()}?
    | {p.GetTokenStream().LT(1).GetText() == "}"}?
    ;

