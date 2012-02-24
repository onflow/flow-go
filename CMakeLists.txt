project(RELIC C)
cmake_minimum_required(VERSION 2.6)

set(PROJECT_VERSION_MAJOR "0")
set(PROJECT_VERSION_MINOR "3")
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
message("   COLOR=[off|on] Build with colored output.")
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
option(COLOR "Build with colored output" on)
option(BIGED "Build with big-endian support" off)
option(SHLIB "Build shared library" on)
option(STLIB "Build static library" on)
option(STBIN "Build static binaries" off)

message(STATUS "Number of times each test or benchmark is ran (default = 50, 1000):")
message("   TESTS=n        If n > 0, build automated tests and run them n times.")
message("   BENCH=n        If n > 0, build automated benchmarks and run them n * n times.\n")

message(STATUS "Number of available processor cores (default = 1):")
message("   CORES=n        If n > 1, build with multithreading support.\n")

message(STATUS "Available architectures (default = X86_64):")
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
message("   WITH=FP       Prime field arithmetic.")
message("   WITH=FB       Binary field arithmetic.")
message("   WITH=EP       Elliptic curves over prime fields.")
message("   WITH=EB       Elliptic curves over binary fields.")
message("   WTTH=EC       Elliptic curve cryptography.")
message("   WITH=HB       Hyperelliptic curve cryptography.")
message("   WITH=PB       Pairings over binary elliptic curves.")
message("   WITH=PP       Pairings over prime elliptic curves.")
message("   WTTH=PC       Pairing-based cryptography.")
message("   WITH=MD       Message digests (Hash functions).")
message("   WITH=CP       Cryptographic protocols.")
message("   WITH=ALL      All of the above.")
message("   Note: the programmer is responsible for not using unselected modules.\n") 

message(STATUS "Available arithmetic backends (default = easy):")
message("   ARITH=easy     Easy-to-understand implementation.")
message("   ARITH=gmp      GNU Multiple Precision library.\n")

message(STATUS "Available memory-allocation policies (default = AUTO):")
message("   ALLOC=AUTO     All memory is automatically allocated.")
message("   ALLOC=STATIC   All memory is allocated statically once.")
message("   ALLOC=DYNAMIC  All memory is allocated dynamically on demand.")
message("   ALLOC=STACK    All memory is allocated from the stack.\n")

message(STATUS "Available pseudo-random number generators (default = FIPS):")
message("   RAND=FIPS      Use the FIPS 186-2 SHA1-based generator. (recommended)\n")

message(STATUS "Available random number generator seeders (default = DEV):")
message("   SEED=ZERO      Use a zero seed. (not crypto safe!)")
message("   SEED=LIBC      Use the libc rand()/random() functions. (not crypto safe!)")
message("   SEED=DEV       Use /dev/random. (recommended)")
message("   SEED=UDEV      Use /dev/urandom.")
message("   SEED=WCGR      Use Windows' CryptGenRandom. (recommended)\n")

message(STATUS "Supported operating systems (default = LINUX):")
message("   OPSYS=NONE     Undefined/No operating system.")
message("   OPSYS=LINUX    GNU/Linux operating system.")
message("   OPSYS=FREEBSD  FreeBSD operating system.")
message("   OPSYS=MACOSX   Mac OS X.")
message("   OPSYS=WINDOWS  Windows operating system.\n")

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
include(cmake/ft.cmake)
include(cmake/ep.cmake)
include(cmake/eb.cmake)
include(cmake/ec.cmake)
include(cmake/hb.cmake)
include(cmake/pp.cmake)
include(cmake/pb.cmake)
include(cmake/pc.cmake)
include(cmake/md.cmake)
include(cmake/cp.cmake) 

# Number of tests and benchmarks
set(BENCH 100 CACHE INTEGER "Number of times each benchmark is ran.")
set(TESTS 100 CACHE INTEGER "Number of times each test is ran.")

# Number of cores.
set(CORES 1 CACHE INTEGER "Number of available processor cores.")

# Architecture and memory layout.
if(CMAKE_SYSTEM_PROCESSOR STREQUAL "x86_64")
	set(ARCH "X86_64" CACHE STRING "Architecture")
	set(WORD 64 CACHE INTEGER "Processor word size")
else(CMAKE_SYSTEM_PROCESSOR STREQUAL "x86_64")
	if(CMAKE_SYSTEM_PROCESSOR STREQUAL "x86")
		set(ARCH "X86" CACHE STRING "Architecture")
		set(WORD 32 CACHE INTEGER "Processor word size")
	endif(CMAKE_SYSTEM_PROCESSOR STREQUAL "x86")
endif(CMAKE_SYSTEM_PROCESSOR STREQUAL "x86_64")
set(ARCH "X86_64" CACHE STRING "Architecture")
set(WORD 64 CACHE INTEGER "Processor word size")
set(ALIGN 1 CACHE INTEGER "Boundary to align digit vectors")

# Default modules.
set(WITH "ALL" CACHE STRING "Selected modules")
LIST(FIND WITH "ALL" TEMP)
if(TEMP GREATER -1)
	set(WITH_BN 1)
	set(WITH_DV 1)
	set(WITH_FP 1)
	set(WITH_FB 1)
	set(WITH_FT 1)	
	set(WITH_EP 1)
	set(WITH_EB 1)
	set(WITH_EC 1)
	set(WITH_HB 1)
	set(WITH_PP 1)
	set(WITH_PB 1)
	set(WITH_PC 1)
	set(WITH_MD 1)
	set(WITH_CP 1)
endif(TEMP GREATER -1)

# See if multiple precision integer arithmetic is required.
LIST(FIND WITH "BN" TEMP)
if(TEMP GREATER -1)
	set(WITH_BN 1)
endif(TEMP GREATER -1)

# See if temporary vectors are required.
LIST(FIND WITH "DV" TEMP)
if(TEMP GREATER -1)
	set(WITH_DV 1)
endif(TEMP GREATER -1)

# See if prime field arithmetic is required.
LIST(FIND WITH "FP" TEMP)
if(TEMP GREATER -1)
	set(WITH_FP 1)
endif(TEMP GREATER -1)

# See if binary field arithmetic is required.
LIST(FIND WITH "FB" TEMP)
if(TEMP GREATER -1)
	set(WITH_FB 1)
endif(TEMP GREATER -1)

# See if ternary field arithmetic is required.
LIST(FIND WITH "FT" TEMP)
if(TEMP GREATER -1)
	set(WITH_FT 1)
endif(TEMP GREATER -1)

# See if prime elliptic curve support is required.
LIST(FIND WITH "EP" TEMP)
if(TEMP GREATER -1)
	set(WITH_EP 1)
endif(TEMP GREATER -1)

# See if binary elliptic curve support is required.
LIST(FIND WITH "EB" TEMP)
if(TEMP GREATER -1)
	set(WITH_EB 1)
endif(TEMP GREATER -1)

# See if elliptic curve cryptography support is required.
LIST(FIND WITH "EC" TEMP)
if(TEMP GREATER -1)
	set(WITH_EC 1)
endif(TEMP GREATER -1)

# See if binary hyperelliptic curve support is required.
LIST(FIND WITH "HB" TEMP)
if(TEMP GREATER -1)
	set(WITH_HB 1)
endif(TEMP GREATER -1)

# See if support for pairings over prime curves is required.
LIST(FIND WITH "PP" TEMP)
if(TEMP GREATER -1)
	set(WITH_PP 1)
endif(TEMP GREATER -1)

# See if support for pairings over binary curves is required.
LIST(FIND WITH "PB" TEMP)
if(TEMP GREATER -1)
	set(WITH_PB 1)
endif(TEMP GREATER -1)

# See if elliptic curve cryptography support is required.
LIST(FIND WITH "PC" TEMP)
if(TEMP GREATER -1)
	set(WITH_PC 1)
endif(TEMP GREATER -1)

# See if support for hash functions is required.
LIST(FIND WITH "MD" TEMP)
if(TEMP GREATER -1)
	set(WITH_MD 1)
endif(TEMP GREATER -1)

# See if support for crptographic protocols is required.
LIST(FIND WITH "CP" TEMP)
if(TEMP GREATER -1)
	set(WITH_CP 1)
endif(TEMP GREATER -1)

# Choose the arithmetic backend.
set(ARITH "easy" CACHE STRING "Arithmetic backend")

# Choose the memory-allocation policy.
set(ALLOC "AUTO" CACHE STRING "Allocation policy")

# Choose the pseudo-random number generator.
set(RAND "FIPS" CACHE STRING "Pseudo-random number generator")

# Choose the pseudo-random number generator.
set(SEED "DEV" CACHE STRING "Random number generator seeder")

# Compiler flags.
set(COMP "$ENV{COMP}" CACHE STRING "User-chosen compiler flags.")
set(COMP "-O2 -funroll-loops -fomit-frame-pointer" CACHE STRING "User-chosen compiler flags.")

# Linker flags.
string(TOLOWER ${ARITH} LFLAGS)
set(LFLAGS "-L${CMAKE_CURRENT_SOURCE_DIR}/src/low/${LFLAGS}/")
set(LINK "$ENV{LINK}" CACHE STRING "User-chosen linker flags.")
set(LFLAGS "${LFLAGS} ${LINK}")

if(STBIN)
	set(LFLAGS "${LFLAGS} -static")
endif(STBIN)

# Discover the operating system.
if(CMAKE_SYSTEM_NAME STREQUAL Linux)
	set(OPSYS "LINUX" CACHE STRING "Operating system")
else(CMAKE_SYSTEM_NAME STREQUAL Linux)
	if(CMAKE_SYSTEM_NAME STREQUAL FreeBSD)
		set(OPSYS "FREEBSD" CACHE STRING "Operating system")
	else(CMAKE_SYSTEM_NAME STREQUAL FreeBSD)
		set(OPSYS "NONE" CACHE STRING "Operating system")
	endif(CMAKE_SYSTEM_NAME STREQUAL FreeBSD)
endif(CMAKE_SYSTEM_NAME STREQUAL Linux)
if(OPSYS STREQUAL LINUX)
	add_definitions(-D_GNU_SOURCE)
endif(OPSYS STREQUAL LINUX)

if(OPSYS STREQUAL LINUX)
	set(TIMER "HPROC" CACHE STRING "Timer")
else(OPSYS STREQUAL LINUX)
	set(TIMER "ANSI" CACHE STRING "Timer")
endif(OPSYS STREQUAL LINUX)

if(DEBUG)
	# If the user did not specify compile flags, we turn off all optimizations.
	set(CFLAGS "-O0 -fno-omit-frame-pointer")
	set(DFLAGS "-ggdb")
else(DEBUG)
	# If the user did not specify compile flags, we use sane defaults.
	set(CFLAGS "${COMP}")
	set(DFLAGS "")
endif(DEBUG)

if(MULTI)
	set(CFLAGS "${CFLAGS} -fopenmp")
endif(MULTI)

if(PROFL)
	set(PFLAGS "-pg -fno-omit-frame-pointer")
else(PROFL)
	set(PFLAGS "")
endif(PROFL)

if(TRACE)
	set(DFLAGS "${DFLAGS} -fPIC -finstrument-functions")
endif(TRACE)

if(ARITH STREQUAL "gmp")
	include(cmake/gmp.cmake)
endif(ARITH STREQUAL "gmp")
if(GMP_FOUND)
	include_directories(${GMP_INCLUDE_DIR})
	set(ARITH_LIBS ${GMP_LIBRARIES})
endif(GMP_FOUND)

set(CMAKE_C_FLAGS "-pipe -std=c99 ${WFLAGS} ${DFLAGS} ${PFLAGS} ${CFLAGS}")
set(CMAKE_EXE_LINKER_FLAGS ${LFLAGS})

message(STATUS "Compiler flags: -pipe -std=c99 -pedantic ${WFLAGS} ${DFLAGS} ${PFLAGS} ${CFLAGS}")
message(STATUS "Linker flags: ${LFLAGS}\n")

string(TOUPPER ${ARITH} ARITH)
configure_file(${CMAKE_CURRENT_SOURCE_DIR}/include/relic_conf.h.in
	${CMAKE_CURRENT_BINARY_DIR}/include/relic_conf.h @ONLY)
message(STATUS "Configured ${CMAKE_CURRENT_SOURCE_DIR}/include/relic_conf.h.in")	
string(TOLOWER ${ARITH} ARITH)

add_subdirectory(${CMAKE_CURRENT_SOURCE_DIR}/src)

if(DOCUM)
	include(cmake/doxygen.cmake)
endif(DOCUM)

if(TESTS GREATER 0)
	enable_testing()
	add_subdirectory(test)
endif(TESTS GREATER 0)

if(BENCH GREATER 0)
	add_subdirectory(bench)
endif(BENCH GREATER 0)
