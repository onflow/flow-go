message(STATUS "Binary field arithmetic configuration (FB module):\n")

message("   *** Arithmetic precision of the binary field module (default = 233):")
message("       FB_POLY=n        The irreducible polynomial size in bits.\n")

message("   *** Available binary field arithmetic methods (default = LRLD4;TABLE;QUICK;BASIC;EXGCD):")
message("       FB_METHD=BASIC    Right-to-left shift-and-add multiplication.")
message("       FB_METHD=INTEG    Integrated modular multiplication.")
message("       FB_METHD=RCOMB    Right-to-left comb multiplication.")
message("       FB_METHD=LCOMB    Left-to-right comb multiplication.")
message("       FB_METHD=LODAH    López-Dahab multiplication with window of width 4.")
message("       FB_METHD=KnMUL    Karatsuba for (n > 0) steps and MUL multiplication.\n")

message("       FB_METHD=BASIC    Bit manipulation squaring.")
message("       FB_METHD=INTEG    Integrated modular squaring.")
message("       FB_METHD=TABLE    Table-based squaring.\n")

message("       FB_METHD=BASIC    Shift-and-add modular reduction.")
message("       FB_METHD=QUICK    Fast reduction modulo a trinomial or pentanomial.\n")

message("       FB_METHD=BASIC    Square root by repeated squaring.")
message("       FB_METHD=QUICK    Fast square root extraction.\n")

message("       FB_METHD=BASIC    Shift-and-add inversion.")
message("       FB_METHD=EXGCD    Inversion by the Extended Euclidean algorithm.")
message("       FB_METHD=ALMOS    Inversion by the Amost inverse algorithm.\n")
message("       Note: these methods must be given in order. Ex: FB_METHD=\"INTEG;TABLE;QUICK;BASIC;ALMOS\"\n")

# Choose the polynomial size.
if (NOT FB_POLYN)
	set(FB_POLYN 283)
endif(NOT FB_POLYN)
set(FB_POLYN ${FB_POLYN} CACHE INTEGER "Irreducible polynomial size in bits.")

# Choose the arithmetic methods.
if (NOT FB_METHD)
	set(FB_METHD "LODAH;TABLE;QUICK;BASIC;EXGCD")
endif(NOT FB_METHD)
list(LENGTH FB_METHD FB_LEN)
if (FB_LEN LESS 5)
	message(FATAL_ERROR "Incomplete FB_METHD specification: ${FB_METHD}")
endif(FB_LEN LESS 5)

list(GET FB_METHD 0 FB_MUL)
list(GET FB_METHD 1 FB_SQR)
list(GET FB_METHD 2 FB_RDC)
list(GET FB_METHD 3 FB_SRT)
list(GET FB_METHD 4 FB_INV)
set(FB_METHD ${FB_METHD} CACHE STRING "Binary field arithmetic method")

# Get the number of Karatsuba steps.
KARAT(${FB_MUL} FB_MUK FB_MUL)