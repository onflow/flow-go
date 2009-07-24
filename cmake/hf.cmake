message(STATUS "Hash functions configuration (HF module):\n")

message("   *** Available hash functions (default = SH256):")
message("       HF_METHD=SHONE    SHA-1 hash function.")
message("       HF_METHD=SH224    SHA-224 hash function.")
message("       HF_METHD=SH256    SHA-256 hash function.")
message("       HF_METHD=SH384    SHA-384 hash function.")
message("       HF_METHD=SH512    SHA-512 hash function.\n")

# Choose the arithmetic methods.
if (NOT HF_METHD)
	set(HF_METHD "SH256")
endif(NOT HF_METHD)
list(LENGTH HF_METHD HF_LEN)
if (HF_LEN LESS 1)
	message(FATAL_ERROR "Incomplete HF_METHD specification: ${HF_METHD}")
endif(HF_LEN LESS 1)

list(GET HF_METHD 0 HF_MAP)
set(HF_METHD ${HF_METHD} CACHE STRING "Hash function.")
