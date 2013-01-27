/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Benchmarks for pairings defined over binary curves.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

#ifdef WITH_EB

static void pairing(void) {
	eb_t p, q;
	fb4_t r;

	fb4_new(r);
	eb_new(p);
	eb_new(q);

#if PB_MAP == ETATS || PB_MAP == ETATN
	BENCH_BEGIN("pb_map_gens1") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map_gens1(r, p, q));
	}
	BENCH_END;
#endif

#if PB_MAP == ETATS || !defined(STRIP)
	BENCH_BEGIN("pb_map_etats") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map_etats(r, p, q));
	}
	BENCH_END;
#endif

#if PB_MAP == ETATN || !defined(STRIP)
	BENCH_BEGIN("pb_map_etatn") {
		eb_rand(p);
		eb_rand(q);
		BENCH_ADD(pb_map_etatn(r, p, q));
	}
	BENCH_END;
#endif

	eb_free(p);
	eb_free(q);
	fb4_free(r);
}

#endif

#ifdef WITH_HB

static void pairing2(void) {
	hb_t p, q;
	fb12_t r;

	fb12_new(r);
	hb_new(p);
	hb_new(q);

#if PB_MAP == ETAT2 || PB_MAP == OETA2
	BENCH_BEGIN("pb_map_gens2") {
		hb_rand(p);
		hb_rand(q);
		BENCH_ADD(pb_map_gens2(r, p, q));
	}
	BENCH_END;
#endif

#if PB_MAP == ETAT2 || !defined(STRIP)
	BENCH_BEGIN("pb_map_etat2 (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (d,n)") {
		hb_rand_deg(p);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (n,d)") {
		hb_rand_deg(p);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, q, p));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_etat2 (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_etat2(r, p, q));
	}
	BENCH_END;
#endif

#if PB_MAP == OETA2 || !defined(STRIP)
	BENCH_BEGIN("pb_map_oeta2 (d,d)") {
		hb_rand_deg(p);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (d,n)") {
		hb_rand_deg(p);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (n,d)") {
		hb_rand_non(p, 0);
		hb_rand_deg(q);
		BENCH_ADD(pb_map_oeta2(r, q, p));
	}
	BENCH_END;

	BENCH_BEGIN("pb_map_oeta2 (n,n)") {
		hb_rand_non(p, 0);
		hb_rand_non(q, 0);
		BENCH_ADD(pb_map_oeta2(r, p, q));
	}
	BENCH_END;
#endif

	hb_free(p);
	hb_free(q);
	fb12_free(r);
}

#endif

int main(void) {
	int r0, r1;
	core_init();
	conf_print();

	util_banner("Benchmarks for the PB module:", 0);

	r0 = r1 = STS_ERR;
#ifdef WITH_EB
	r0 = eb_param_set_any_super();
	if (r0 == STS_OK) {
		eb_param_print();
		util_banner("Arithmetic:", 1);
		pairing();
	}
#endif

#ifdef WITH_HB
	r1 = hb_param_set_any_super();
	if ( r1 == STS_OK) {
		hb_param_print();
		util_banner("Arithmetic:", 1);
		pairing2();
	}
#endif

	if (r0 == STS_ERR && r1 == STS_ERR) {
		THROW(ERR_NO_CURVE);
	}

	core_clean();
	return 0;
}
