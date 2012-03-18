message(STATUS "Ternary field arithmetic configuration (FT module):\n")

message("   ** Options for the binary elliptic curve module (default = 509,0,on):")
message("      FT_POLYN=n        The irreducible polynomial size in trits.")
message("      FT_KARAT=n        The number of Karatsuba levels.") 
message("      FT_TRINO=[off|on] Prefer trinomials.\n")

message("   ** Available ternary field arithmetic methods (default = LODAH;TABLE;QUICK;BASIC;EXGCD):")
message("      FT_METHD=BASIC    Right-to-left shift-and-add multiplication.")
message("      FT_METHD=INTEG    Integrated modular multiplication.")
message("      FT_METHD=LODAH    López-Dahab comb multiplication with window of width 4.\n")

message("      FT_METHD=BASIC    Bit manipulation cubing.")
message("      FT_METHD=INTEG    Integrated modular cubing.")
message("      FT_METHD=TABLE    Table-based cubing.\n")

message("      FT_METHD=BASIC    Shift-and-add modular reduction.")
message("      FT_METHD=QUICK    Fast reduction modulo a trinomial or pentanomial.\n")

message("      FT_METHD=BASIC    Cube root by repeated cubing.")
message("      FT_METHD=QUICK    Fast cube root extraction.\n")

message("      FT_METHD=BASIC    Inversion by Fermat's Little Theorem.")
message("      FT_METHD=BINAR    Binary Inversion algorithm.")
message("      FT_METHD=EXGCD    Inversion by the Extended Euclidean algorithm.")
message("      FT_METHD=ALMOS    Inversion by the Amost inverse algorithm.\n")
message("      Note: these methods must be given in order. Ex: FT_METHD=\"INTEG;INTEG;QUICK;QUICK;ALMOS\"\n")

# Choose the polynomial size.
if (NOT FT_POLYN)
	set(FT_POLYN 509)
endif(NOT FT_POLYN)
set(FT_POLYN ${FT_POLYN} CACHE INTEGER "Irreducible polynomial size in bits.")

# Fix the number of Karatsuba instances
if (NOT FT_KARAT)
	set(FT_KARAT 0)
endif(NOT FT_KARAT)
set(FT_KARAT ${FT_KARAT} CACHE INTEGER "Number of Karatsuba levels.")

option(FT_TRINO "Prefer trinomials." on)
option(FT_PRECO "Precompute multiplication table for sqrt(z)." on)

# Choose the arithmetic methods.
if (NOT FT_METHD)
	set(FT_METHD "LODAH;TABLE;QUICK;BASIC;EXGCD")
endif(NOT FT_METHD)
list(LENGTH FT_METHD FT_LEN)
if (FT_LEN LESS 5)
	message(FATAL_ERROR "Incomplete FT_METHD specification: ${FT_METHD}")
endif(FT_LEN LESS 5)

list(GET FT_METHD 0 FT_MUL)
list(GET FT_METHD 1 FT_CUB)
list(GET FT_METHD 2 FT_RDC)
list(GET FT_METHD 3 FT_CRT)
list(GET FT_METHD 4 FT_INV)

set(FT_METHD ${FT_METHD} CACHE STRING "Method for ternary field arithmetic.")
