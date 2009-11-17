macro(ADD_MODULE MODULE)
	add_executable(bench_${MODULE} bench_${MODULE}.c)
	if (STLIB)
		target_link_libraries(bench_${MODULE} relic_s)
	endif(STLIB)
	if (STBIN)
		set_target_properties(bench_${MODULE} PROPERTIES LINK_FLAGS "-static")
		set_target_properties(bench_${MODULE} PROPERTIES LINK_SEARCH_END_STATIC 1)
	endif(STBIN)
	add_test(bench_${MODULE} ${EXECUTABLE_OUTPUT_PATH}/bench_${MODULE})
endmacro(ADD_MODULE)

if (CHECK)
	ADD_MODULE(err)
endif(CHECK)

if (WITH_BN)
	ADD_MODULE(bn)
endif(WITH_BN)

if (WITH_DV)
	ADD_MODULE(dv)
endif(WITH_DV)

if (WITH_FP)
	ADD_MODULE(fp)
endif(WITH_FP)

if (WITH_EP)
	ADD_MODULE(ep)
endif(WITH_EP)

if (WITH_FB)
	ADD_MODULE(fb)
endif(WITH_FB)

if (WITH_EB)
	ADD_MODULE(eb)
endif(WITH_EB)

if (WITH_PB)
	ADD_MODULE(pb)
endif(WITH_PB)

if (WITH_RATE)
	ADD_MODULE(pp)
endif(WITH_RATE)

if (WITH_CP)
	ADD_MODULE(cp)
endif(WITH_CP)
