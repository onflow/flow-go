/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Benchmarks for pairings defined over prime elliptic curves.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

static void pairing(void) {
	ep2_t p;
	ep_t q;
	fp12_t e;
	bn_t k, n, l;

	ep2_null(p);
	ep_null(q);
	bn_null(k);
	bn_null(n);
	bn_null(l);
	fp12_null(e);

	ep2_new(p);
	ep_new(q);
	bn_new(k);
	bn_new(n);
	bn_new(l);
	fp12_new(e);

	ep2_curve_get_ord(n);

	BENCH_BEGIN("pp_dbl_k12") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_dbl_k12(e, p, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("pp_dbl_k12_basic") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_dbl_k12_basic(e, p, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("pp_dbl_k12_projc_basic") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_dbl_k12_projc_basic(e, p, p, q));
	}
	BENCH_END;
#endif

#if PP_EXT == LAZYR || !defined(STRIP)
	BENCH_BEGIN("pp_dbl_k12_projc_lazyr") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_dbl_k12_projc_lazyr(e, p, p, q));
	}
	BENCH_END;
#endif

#endif

	BENCH_BEGIN("pp_add") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_add_k12(e, p, p, q));
	}
	BENCH_END;

#if EP_ADD == BASIC || !defined(STRIP)
	BENCH_BEGIN("pp_add_k12_basic") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_add_k12_basic(e, p, p, q));
	}
	BENCH_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)

#if PP_EXT == BASIC || !defined(STRIP)
	BENCH_BEGIN("pp_add_k12_projc_basic") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_add_k12_projc_basic(e, p, p, q));
	}
	BENCH_END;
#endif

#if PP_EXT == LAZYR || !defined(STRIP)
	BENCH_BEGIN("pp_add_k12_projc_lazyr") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_add_k12_projc_lazyr(e, p, p, q));
	}
	BENCH_END;
#endif

#endif

	BENCH_BEGIN("pp_exp") {
		fp12_rand(e);
		BENCH_ADD(pp_exp(e, e));
	}
	BENCH_END;

	BENCH_BEGIN("pp_map") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_map(e, q, p));
	}
	BENCH_END;

#if PP_MAP == TATEP || !defined(STRIP)
	BENCH_BEGIN("pp_map_tatep") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_map_tatep(e, q, p));
	}
	BENCH_END;
#endif

#if PP_MAP == WEILP || !defined(STRIP)
	BENCH_BEGIN("pp_map_weilp") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_map_weilp(e, q, p));
	}
	BENCH_END;
#endif

#if PP_MAP == OATEP || !defined(STRIP)
	BENCH_BEGIN("pp_map_oatep") {
		ep2_rand(p);
		ep_rand(q);
		BENCH_ADD(pp_map_oatep(e, q, p));
	}
	BENCH_END;
#endif

	ep2_free(p);
	ep_free(q);
	bn_free(k);
	bn_free(n);
	bn_free(l);
	fp12_free(e);
}

int main(void) {
	core_init();
	conf_print();

	util_banner("Benchmarks for the PP module:", 0);

	if (ep_param_set_any_pairf() != STS_OK) {
		THROW(ERR_NO_CURVE);
		core_clean();
		return 0;
	}

	ep_param_print();
	util_banner("Arithmetic:", 1);
	pairing();

	core_clean();
	return 0;
}
