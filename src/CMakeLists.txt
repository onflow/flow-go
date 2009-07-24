string(TOLOWER ${ARITH} ARITH_PATH)

if (EXISTS "${CMAKE_CURRENT_SOURCE_DIR}/low/${ARITH_PATH}/CMakeLists.txt")
	include("${CMAKE_CURRENT_SOURCE_DIR}/low/${ARITH_PATH}/CMakeLists.txt")
endif(EXISTS "${CMAKE_CURRENT_SOURCE_DIR}/low/${ARITH_PATH}/CMakeLists.txt")	

set(CORE_SRCS relic_core.c relic_rand.c relic_conf.c relic_pool.c relic_util.c)
file(GLOB BN_SRCS bn/*.c)
file(GLOB DV_SRCS dv/*.c)
file(GLOB FP_SRCS fp/*.c)
file(GLOB FB_SRCS fb/*.c)
file(GLOB EP_SRCS ep/*.c)
file(GLOB EB_SRCS eb/*.c)
file(GLOB PP_SRCS pp/*.c ep2/*.c)
file(GLOB PB_SRCS pb/*.c)
file(GLOB HF_SRCS hf/*.c)

set(RELIC_SRCS ${CORE_SRCS})

if (CHECK)
	list(APPEND RELIC_SRCS "relic_error.c")
endif(CHECK)

if (TRACE)
	list(APPEND RELIC_SRCS "relic_trace.c")
endif(TRACE)	

if (TESTS GREATER 0)
	list(APPEND RELIC_SRCS "relic_test.c")
endif(TESTS GREATER 0)

if (BENCH GREATER 0)
	list(APPEND RELIC_SRCS "relic_bench.c")
endif(BENCH GREATER 0)

if (WITH_BN)
	list(APPEND RELIC_SRCS ${BN_SRCS})
	file(GLOB TEMP low/easy/relic_bn*.c)
	list(APPEND LOW_SRCS ${TEMP})
endif(WITH_BN)

if (WITH_DV)
	list(APPEND RELIC_SRCS ${DV_SRCS})
endif(WITH_DV)

if (WITH_FP)
	list(APPEND RELIC_SRCS ${FP_SRCS})
	file(GLOB TEMP low/easy/relic_fp*.c)
	list(APPEND LOW_SRCS ${TEMP})
endif(WITH_FP)

if (WITH_FB)
	list(APPEND RELIC_SRCS ${FB_SRCS})
	file(GLOB TEMP low/easy/relic_fb*.c)
	list(APPEND LOW_SRCS ${TEMP})
endif(WITH_FB)

if (WITH_EP)
	list(APPEND RELIC_SRCS ${EP_SRCS})
endif(WITH_EP)

if (WITH_EB)
	list(APPEND RELIC_SRCS ${EB_SRCS})
endif(WITH_EB)

if (WITH_PP)
	list(APPEND RELIC_SRCS ${PP_SRCS})
endif(WITH_PP)

if (WITH_PB)
	list(APPEND RELIC_SRCS ${PB_SRCS})
endif(WITH_PB)

if (WITH_CP)
	if (WITH_BN)
		list(APPEND RELIC_SRCS "cp/relic_cp_rsa.c")
	endif(WITH_BN)
	if (WITH_EB)
		list(APPEND RELIC_SRCS "cp/relic_cp_ecdsa.c")
	endif(WITH_EB)
	if (WITH_PB)
		list(APPEND RELIC_SRCS "cp/relic_cp_sokaka.c")
	endif(WITH_PB)
endif(WITH_CP)

if (WITH_HF)
	list(APPEND RELIC_SRCS ${HF_SRCS})
endif(WITH_HF)

foreach(SRC ${LOW_SRCS})
	get_filename_component(SRC ${SRC} NAME_WE)
	set(FILE "${CMAKE_CURRENT_SOURCE_DIR}/low/${ARITH_PATH}/${SRC}")
	if (EXISTS "${FILE}.c")
		list(APPEND ARITH_SRCS "${FILE}.c")
	endif(EXISTS "${FILE}.c")
	if (EXISTS "${FILE}.s")
		list(APPEND ARITH_ASMS "${FILE}.s")
	endif(EXISTS "${FILE}.s")
	if(NOT EXISTS "${FILE}.c" AND NOT EXISTS "${FILE}.s")
		list(APPEND ARITH_SRCS "low/easy/${SRC}.c")
	endif(NOT EXISTS "${FILE}.c" AND NOT EXISTS "${FILE}.s")
endforeach(SRC)

list(APPEND RELIC_SRCS ${ARITH_SRCS})

foreach(INC ${INCLUDE})
	set(RELIC_INCS -I ${INC} ${RELIC_INCS})
endforeach(INC)

foreach(SRC ${ARITH_ASMS})
	get_filename_component(FILE ${SRC} NAME_WE)
	set(OBJ ${CMAKE_CURRENT_BINARY_DIR}/${FILE}.o)
	add_custom_command(OUTPUT ${OBJ}
		MAIN_DEPENDENCY ${SRC}
		COMMAND ${CMAKE_C_COMPILER} ${RELIC_INCS} -DASM -x assembler-with-cpp -o ${OBJ} -c ${SRC})
		set(ARITH_OBJS ${ARITH_OBJS} ${OBJ})
endforeach(SRC)

add_custom_target(arith_objs DEPENDS ${ARITH_OBJS})
add_dependencies(arith_objs ${CMAKE_CURRENT_BINARY_DIR}/include/relic_conf.h)

if (SHLIB)
	add_library(relic SHARED ${RELIC_SRCS} ${ARITH_OBJS})
	add_dependencies(relic arith_objs)
	target_link_libraries(relic ${ARITH_LIBS})	
endif(SHLIB)	

if (STLIB)
	add_library(relic_s STATIC ${RELIC_SRCS} ${ARITH_OBJS})
	add_dependencies(relic_s arith_objs)
	target_link_libraries(relic_s ${ARITH_LIBS})	
endif(STLIB)
