project(RELIC)
cmake_minimum_required(VERSION 2.6)

set(PROJECT_VERSION_MAJOR "0")
set(PROJECT_VERSION_MINOR "1")
set(PROJECT_VERSION_PATCH "1")
set(PROJECT_VERSION "${PROJECT_VERSION_MAJOR}.${PROJECT_VERSION_MINOR}.${PROJECT_VERSION_PATCH}")
set(VERSION ${PROJECT_VERSION})

set(INCLUDE ${CMAKE_CURRENT_BINARY_DIR}/include ${CMAKE_CURRENT_SOURCE_DIR}/include ${CMAKE_CURRENT_SOURCE_DIR}/include/low)
include_directories(${INCLUDE})

set(LIBRARY_OUTPUT_PATH ${PROJECT_BINARY_DIR}/lib)
set(EXECUTABLE_OUTPUT_PATH ${PROJECT_BINARY_DIR}/bin)
  
set(WFLAGS "-Wall")

message("\n-- Configuring ${PROJECT_NAME} ${PROJECT_VERSION}...\n")

message(STATUS "Available switches (default = CHECK, VERBS, DOCUM):")
message("   DEBUG=[off|on] Build with debugging support.")
message("   PROFL=[off|on] Build with profiling support.")
message("   CHECK=[off|on] Build with error-checking support.")
message("   VERBS=[off|on] Build with detailed error messages.")
message("   TRACE=[off|on] Build with tracing support.")
message("   MULTI=[off|on] Build with multithreading.")
message("   DOCUM=[off|on] Build documentation.")
message("   STRIP=[off|on] Build only selected algorithms.")
message("   QUIET=[off|on] Build with printing disabled.")
message("   BIGED=[off|on] Build with big-endian support.") 
message("   SHLIB=[off|on] Build shared library.")
message("   STLIB=[off|on] Build static library.")
message("   STBIN=[off|on] Build static binaries.\n")

option(DEBUG "Build with debugging support" off)
option(PROFL "Build with debugging support" off)
option(CHECK "Build with error-checking support" on)
option(VERBS "Build with detailed error messages" on)
option(TRACE "Build with tracing support" off)
option(DOCUM "Build documentation" on)
option(MULTI "Build with multithreading" off)
option(STRIP "Build only the selected algorithms" off)
option(QUIET "Build with printing disabled" off)
option(BIGED "Build with big-endian support" off)
option(SHLIB "Build shared library" on)
option(STLIB "Build static library" on)
option(STBIN "Build static binaries" off)

message(STATUS "Number of times each test or benchmark is ran (default = 50, 1000):")
message("   TESTS=n        If n > 0, build automated tests and run them n times.")
message("   BENCH=n        If n > 0, build automated benchmarks and run them n * n times.\n")

message(STATUS "Number of available processor cores (default = 1):")
message("   CORES=n        If n > 1, build with multithreading support.\n")

message(STATUS "Available architectures (default = X86):")
message("   ARCH=AVR       Atmel AVR ATMega128 8-bit architecture.")
message("   ARCH=MSP       TI MSP430 16-bit architecture.")
message("   ARCH=ARM       ARM 32-bit architecture.")
message("   ARCH=X86       Intel x86-compatible 32-bit architecture.")
message("   ARCH=X86_64    AMD x86_64-compatible 64-bit architecture.\n")

message(STATUS "Available word sizes (default = 64):")
message("   WORD=8         Build a 8-bit library.")
message("   WORD=16        Build a 16-bit library.")
message("   WORD=32        Build a 32-bit library.")
message("   WORD=64        Build a 64-bit library.\n")

message(STATUS "Byte boundary to align digit vectors (default = 1):")
message("   ALIGN=1        Do not align digit vectors.")
message("   ALIGN=2        Align digit vectors into 16-bit boundaries.")
message("   ALIGN=8        Align digit vectors into 64-bit boundaries.")
message("   ALIGN=16       Align digit vectors into 128-bit boundaries.\n")

message(STATUS "Available modules (default = ALL)")
message("   WITH=BN       Multiple precision arithmetic.")
message("   WITH=DV       Temporary double-precision digit vectors.")
message("   WITH=FB       Binary field arithmetic.")
message("   WITH=EB       Elliptic curves over binary fields.")
message("   WITH=PB       Pairings over binary elliptic curves.")
message("   WITH=HF       Hash functions.")
message("   WITH=CP       Cryptographic protocols.")
message("   WITH=ALL      All of the above.")
message("   Note: the programmer is responsible for not using unselected modules.\n") 

message(STATUS "Available arithmetic backends (default = EASY):")
message("   ARITH=EASY     Easy-to-understand implementation.")
message("   ARITH=GMP      GNU Multiple Precision library.\n")

message(STATUS "Available memory-allocation policies (default = STACK):")
message("   ALLOC=STATIC   All memory is allocated statically once.")
message("   ALLOC=DYNAMIC  All memory is allocated dynamically on demand.")
message("   ALLOC=STACK    All memory is allocated from the stack.\n")

message(STATUS "Available pseudo-random number generators (default = FIPS):")
message("   RAND=FIPS      Use the FIPS 186-2 SHA1-based generator. (recommended)\n")

message(STATUS "Available random number generator seeders (default = DEV):")
message("   SEED=LIBC      Use the libc rand()/random() functions. (not crypto safe!)")
message("   SEED=DEV       Use /dev/random. (recommended)\n")

message(STATUS "Supported operating systems (default = LINUX):")
message("   OPSYS=NONE     Undefined/No operating system.")
message("   OPSYS=LINUX    GNU/Linux operating system.")
message("   OPSYS=FREEBSD  FreeBSD operating system.\n")

message(STATUS "Supported timers (default = HIGH):")
message("   TIMER=NONE     No timer.")
message("   TIMER=HREAL    GNU/Linux realtime high-resolution timer.")
message("   TIMER=HPROC    GNU/Linux per-process high-resolution timer.")
message("   TIMER=HTHRD    GNU/Linux per-thread high-resolution timer.")
message("   TIMER=ANSI     ANSI-compatible timer.")
message("   TIMER=POSIX    POSIX-compatible timer.")
message("   TIMER=CYCLE    Cycle-counting timer. (architecture-dependant)\n")

include(cmake/bn.cmake)
include(cmake/fp.cmake)
include(cmake/fb.cmake)
include(cmake/eb.cmake)
include(cmake/pb.cmake)
include(cmake/hf.cmake)
include(cmake/cp.cmake) 

# Number of tests and benchmarks
if (NOT DEFINED BENCH)
	set(BENCH 100)
endif(NOT DEFINED BENCH)
set(BENCH "${BENCH}" CACHE STRING "Number of times each benchmark is ran.")

if (NOT DEFINED TESTS)
	set(TESTS 100)
endif(NOT DEFINED TESTS)
set(TESTS "${TESTS}" CACHE STRING "Number of times each test is ran.")

if (NOT DEFINED CORES)
	set(CORES 1)
endif(NOT DEFINED CORES)
set(CORES "${CORES}" CACHE STRING "Number of available processor cores.")

# Default Architecture.
if (NOT ARCH)
	set(ARCH	"X86_64")
endif(NOT ARCH)
set(ARCH ${ARCH} CACHE STRING "Architecture")

# Default word size.
if (NOT WORD)
	set(WORD 64)
endif(NOT WORD)
set(WORD ${WORD} CACHE INTEGER "Processor word size")

if (NOT ALIGN)
	set(ALIGN 1)
endif(NOT ALIGN)	
set(ALIGN ${ALIGN} CACHE INTEGER "Boundary to align digit vectors")

# Default modules.
if (NOT WITH)
	set(WITH "ALL")
endif(NOT WITH)
set(WITH "${WITH}" CACHE STRING "Selected modules")

LIST(FIND WITH "ALL" TEMP)
if (TEMP GREATER -1)
	set(WITH_BN 1)
	set(WITH_DV 1)
	set(WITH_FP 1)
	set(WITH_FB 1)
	set(WITH_EB 1)
	set(WITH_PB 1)
	set(WITH_HF 1)
	set(WITH_CP 1)
endif(TEMP GREATER -1)

# See if multiple precision integer arithmetic is required.
LIST(FIND WITH "BN" TEMP)
if (TEMP GREATER -1)
	set(WITH_BN 1)
endif(TEMP GREATER -1)

# See if temporary vectors are required.
LIST(FIND WITH "DV" TEMP)
if (TEMP GREATER -1)
	set(WITH_DV 1)
endif(TEMP GREATER -1)

# See if prime field arithmetic is required.
LIST(FIND WITH "FP" TEMP)
if (TEMP GREATER -1)
	set(WITH_FP 1)
endif(TEMP GREATER -1)

# See if binary field arithmetic is required.
LIST(FIND WITH "FB" TEMP)
if (TEMP GREATER -1)
	set(WITH_FB 1)
endif(TEMP GREATER -1)

# See if prime elliptic curve support is required.
LIST(FIND WITH "EP" TEMP)
if (TEMP GREATER -1)
	set(WITH_EP 1)
endif(TEMP GREATER -1)

# See if binary elliptic curve support is required.
LIST(FIND WITH "EB" TEMP)
if (TEMP GREATER -1)
	set(WITH_EB 1)
endif(TEMP GREATER -1)

# See if support for pairings over prime curves is required.
LIST(FIND WITH "PP" TEMP)
if (TEMP GREATER -1)
	set(WITH_PP 1)
endif(TEMP GREATER -1)

# See if support for pairings over binary curves is required.
LIST(FIND WITH "PB" TEMP)
if (TEMP GREATER -1)
	set(WITH_PB 1)
endif(TEMP GREATER -1)

# See if support for hash functions is required.
LIST(FIND WITH "HF" TEMP)
if (TEMP GREATER -1)
	set(WITH_HF 1)
endif(TEMP GREATER -1)

# See if support for crptographic protocols is required.
LIST(FIND WITH "CP" TEMP)
if (TEMP GREATER -1)
	set(WITH_CP 1)
endif(TEMP GREATER -1)

# Choose the arithmetic backend.
if (NOT ARITH)
	set(ARITH "easy")
endif(NOT ARITH)
set(ARITH "${ARITH}" CACHE STRING "Arithmetic backend")

# Choose the memory-allocation policy.
if (NOT ALLOC)
	set(ALLOC "STACK")
endif(NOT ALLOC)
set(ALLOC ${ALLOC} CACHE STRING "Allocation policy")

# Choose the pseudo-random number generator.
if (NOT RAND)
	set(RAND "FIPS")
endif(NOT RAND)
set(RAND ${RAND} CACHE STRING "Pseudo-random number generator")

# Choose the pseudo-random number generator.
if (NOT SEED)
	set(SEED "DEV")
endif(NOT SEED)
set(SEED ${SEED} CACHE STRING "Random number generator seeder")

set (TMP "$ENV{COMP}")
if (TMP)
	set(COMP ${TMP} CACHE STRING "User-chosen compiler flags.")
endif(TMP)

if (NOT COMP)
	set(COMP "-O2 -funroll-loops -fomit-frame-pointer" CACHE STRING "User-chosen compiler flags.")
endif(NOT COMP)

string(TOLOWER ${ARITH} LFLAGS)
set(LFLAGS "-L ${CMAKE_CURRENT_SOURCE_DIR}/src/low/${LFLAGS}/")
set (TMP "$ENV{LINK}")
if (TMP)
	set(LINK ${TMP} CACHE STRING "User-chosen linker flags.")
    set(LFLAGS "${LFLAGS} ${LINK}")
endif(TMP)

if (NOT OPSYS)
	# Discover the operating system.
	if (CMAKE_SYSTEM_NAME STREQUAL Linux)
		set(OPSYS "LINUX")
	else(CMAKE_SYSTEM_NAME STREQUAL Linux)
		if (CMAKE_SYSTEM_NAME STREQUAL FreeBSD)
			set(OPSYS "FREEBSD")
		endif (CMAKE_SYSTEM_NAME STREQUAL FreeBSD)
	endif(CMAKE_SYSTEM_NAME STREQUAL Linux)
endif(NOT OPSYS)
if (OPSYS STREQUAL LINUX)
	add_definitions(-D_GNU_SOURCE)
	set(LFLAGS "${LFLAGS} -lrt")
endif(OPSYS STREQUAL LINUX)
set(OPSYS ${OPSYS} CACHE STRING "Operating system")

if (NOT TIMER)
	set(TIMER "HPROC")
endif(NOT TIMER)
set(TIMER ${TIMER} CACHE STRING "Timer")

if (DEBUG)
	# If the user did not specify compile flags, we turn off all optimizations.
	set(CFLAGS "-O0 -fno-omit-frame-pointer")
	set(DFLAGS "-ggdb")
else(DEBUG)
	# If the user did not specify compile flags, we use sane defaults.
	set(CFLAGS "${COMP}")
	set(DFLAGS "")
endif(DEBUG)

if (MULTI)
	set(CFLAGS "${CFLAGS} -fopenmp")
	set(LFLAGS "${LFLAGS} -lgomp -lm")
endif(MULTI)

if (PROFL)
	set(PFLAGS "-pg -fno-omit-frame-pointer")
else(PROFL)
	set(PFLAGS "")
endif(PROFL)

if (CHECK)
	set(DOC_OPTIONS "${DOC_OPTIONS} CHECK=1")
endif(CHECK)

if (VERBS)
	set(DOC_OPTIONS "${DOC_OPTIONS} VERBS=1")
endif(VERBS)

if (TRACE)
	set(DFLAGS "${DFLAGS} -fPIC -finstrument-functions")
	set(LFLAGS "${LFLAGS} -ldl")
	set(DOC_OPTIONS "${DOC_OPTIONS} TRACE=1")
endif(TRACE)

if (STRIP)
	set(DOC_OPTIONS "${DOC_OPTIONS} STRIP=1")
endif(STRIP)

if (QUIET)
	set(DOC_OPTIONS "${DOC_OPTIONS} QUIET=1")
endif(QUIET)

if (ARITH STREQUAL "gmp")
	include(cmake/gmp.cmake)
	set(LFLAGS "${LFLAGS} -lgmp")
endif(ARITH STREQUAL "gmp")

set(CMAKE_C_FLAGS "-pipe -std=c99 -pedantic ${WFLAGS} ${DFLAGS} ${PFLAGS} ${CFLAGS}")
set(CMAKE_EXE_LINKER_FLAGS ${LFLAGS})

message(STATUS "Compiler flags: -pipe -std=c99 -pedantic ${WFLAGS} ${DFLAGS} ${PFLAGS} ${CFLAGS}")
message(STATUS "Linker flags: ${LFLAGS}\n")

string(TOUPPER ${ARITH} ARITH)
configure_file(${CMAKE_CURRENT_SOURCE_DIR}/include/relic_conf.h.in
	${CMAKE_CURRENT_BINARY_DIR}/include/relic_conf.h @ONLY)
message(STATUS "Configured ${CMAKE_CURRENT_SOURCE_DIR}/include/relic_conf.h.in")	
string(TOLOWER ${ARITH} ARITH)

add_subdirectory(${CMAKE_CURRENT_SOURCE_DIR}/src)

if (DOCUM)
	include(cmake/doxygen.cmake)
endif(DOCUM)

if (TESTS GREATER 0)
	enable_testing()
	add_subdirectory(test)
endif(TESTS GREATER 0)

if (BENCH GREATER 0)
	add_subdirectory(bench)
endif(BENCH GREATER 0)
