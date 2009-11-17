message(STATUS "Prime field arithmetic configuration (FP module):\n")

message("   ** Arithmetic precision of the prime field module (default = BITS):")
message("      FP_PRIME=n        The prime modulus size in bits.\n")
message("      FP_KARAT=n        The number of Karatsuba levels.")
message("      FP_PMERS=[off|on] Prefer Pseudo-Mersenne primes over random primes.\n")

message("   ** Available prime field arithmetic methods (default = COMBA;COMBA;MONTY):")
message("      FP_METHD=BASIC    Schoolbook multiplication.\n")
message("      FP_METHD=INTEG    Integrated modular multiplication.")
message("      FP_METHD=COMBA    Comba multiplication.")

message("      FP_METHD=BASIC    Schoolbook squaring.")
message("      FP_METHD=INTEG    Integrated modular squaring.")
message("      FP_METHD=COMBA    Comba squaring.\n")

message("      FP_METHD=BASIC    Division-based reduction.")
message("      FP_METHD=QUICK    Fast reduction modulo special form prime (2^t - c, c > 0).")
message("      FP_METHD=MONTY    Montgomery modular reduction.\n")
message("      Note: these methods must be given in order. Ex: FP_METHD=\"BASIC;COMBA;MONTY\"\n")

# Choose the prime field size.
if (NOT FP_PRIME)
	set(FP_PRIME 256)
endif(NOT FP_PRIME)
set(FP_PRIME ${FP_PRIME} CACHE INTEGER "Prime modulus size")

# Fix the number of Karatsuba instances
if (NOT FP_KARAT)
	set(FP_KARAT 0)
endif(NOT FP_KARAT)
set(FP_KARAT ${FP_KARAT} CACHE INTEGER "Number of Karatsuba levels.")

option(FP_PMERS "Prefer special form primes over random primes." on)

# Choose the arithmetic methods.
if (NOT FP_METHD)
	set(FP_METHD "COMBA;COMBA;MONTY")
endif(NOT FP_METHD)
list(LENGTH FP_METHD FP_LEN)
if (FP_LEN LESS 3)
	message(FATAL_ERROR "Incomplete FP_METHD specification: ${FP_METHD}")
endif(FP_LEN LESS 3)

list(GET FP_METHD 0 FP_MUL)
list(GET FP_METHD 1 FP_SQR)
list(GET FP_METHD 2 FP_RDC)
set(FP_METHD ${FP_METHD} CACHE STRING "Prime field arithmetic method")
