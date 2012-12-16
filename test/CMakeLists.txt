macro(ADD_MODULE MODULE)
	add_executable(test_${MODULE} test_${MODULE}.c)
	if (STLIB)
		target_link_libraries(test_${MODULE} relic_s)
	else(STLIB)
		if (SHLIB)
			target_link_libraries(test_${MODULE} relic)
		endif(SHLIB)
	endif(STLIB)
	if (STBIN)
		set_target_properties(test_${MODULE} PROPERTIES LINK_SEARCH_END_STATIC 1)
	endif(STBIN)
	add_test(test_${MODULE} ${SIMP} ${SIMA} ${EXECUTABLE_OUTPUT_PATH}/test_${MODULE})
endmacro(ADD_MODULE)

if (CHECK)
	ADD_MODULE(error)
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

if (WITH_FB)
	ADD_MODULE(fb)
endif(WITH_FB)

if (WITH_FT)
	ADD_MODULE(ft)
endif(WITH_FT)

if (WITH_EP)
	ADD_MODULE(ep)
endif(WITH_EP)

if (WITH_EB)
	ADD_MODULE(eb)
endif(WITH_EB)

if (WITH_EC)
	ADD_MODULE(ec)
endif(WITH_EC)

if (WITH_PC)
	ADD_MODULE(pc)
endif(WITH_PC)

if (WITH_HB)
	ADD_MODULE(hb)
endif(WITH_HB)

if (WITH_PP)
	ADD_MODULE(fpx)
	ADD_MODULE(epx)
	ADD_MODULE(pp)
endif(WITH_PP)	

if (WITH_PB)
	ADD_MODULE(fbx)
	ADD_MODULE(pb)
endif(WITH_PB)

if (WITH_MD)
	ADD_MODULE(md)
endif(WITH_MD)

if (WITH_CP)
	ADD_MODULE(cp)
endif(WITH_CP)

ADD_MODULE(rand)