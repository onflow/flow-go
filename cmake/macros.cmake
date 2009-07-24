# This macro reads the number of Karatsuba steps from a method specification.
macro(KARAT METHD STEPS END)
	string(REGEX REPLACE "K([0-9]*).*" "\\1" ${STEPS} ${METHD})
	string(REGEX REPLACE "K[0-9]*(.*)" "\\1" ${END} ${METHD})
	if (NOT ${STEPS} OR ${STEPS} STREQUAL ${END})
		set(${STEPS} 0)
	endif(NOT ${STEPS} OR ${STEPS} STREQUAL ${END})
endmacro(KARAT)

# This macro reads the width size of a method specification.
macro(WIDTH METHD ALG W)
	string(REGEX REPLACE "(.*)[1-6]" "\\1W" ${ALG} ${METHD})
	string(REGEX REPLACE ".*([1-6])" "\\1" ${W} ${METHD})	
	if (NOT ${W} OR ${ALG} STREQUAL ${W})
		set(${W} 0)
	endif(NOT ${W} OR ${ALG} STREQUAL ${W})
endmacro(WIDTH)
