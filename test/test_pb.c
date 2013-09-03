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
 * Tests for pairings defined over binary curves.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

static int pairing1(void) {
	int code = STS_ERR;
	fb4_t e1, e2;
	eb_t p, q, r;
	bn_t k, n;

	fb4_null(e1);
	fb4_null(e2);
	eb_null(p);
	eb_null(q);
	eb_null(r);
	bn_null(k);
	bn_null(n);

	TRY {
		fb4_new(e1);
		fb4_new(e2);
		eb_new(p);
		eb_new(q);
		eb_new(r);
		bn_new(k);
		bn_new(n);

		eb_curve_get_ord(n);

#if PB_MAP == ETATS || PB_MAP == ETATN
		TEST_BEGIN("pairing is non-degenerate") {
			eb_rand(p);
			eb_rand(q);
			pb_map(e1, p, q);
			fb4_zero(e2);
			fb_set_dig(e2[0], 1);
			TEST_ASSERT(fb4_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("pairing is bilinear") {
			eb_rand(p);
			eb_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			eb_mul(r, q, k);
			fb4_zero(e1);
			fb4_zero(e2);
			pb_map(e1, p, r);
			TEST_ASSERT(!bn_is_zero(n), end);
			eb_mul(r, p, k);
			pb_map(e2, r, q);
			TEST_ASSERT(fb4_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif

#if PB_MAP == ETATS || !defined(STRIP)
		TEST_BEGIN("etat pairing with square roots is correct") {
			eb_rand(p);
			eb_rand(q);
			pb_map(e1, p, q);
			pb_map_etats(e2, p, q);
			TEST_ASSERT(fb4_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif

#if PB_MAP == ETATN || !defined(STRIP)
		TEST_BEGIN("etat pairing without square roots is correct") {
			eb_rand(p);
			eb_rand(q);
			pb_map(e1, p, q);
			pb_map_etatn(e2, p, q);
			TEST_ASSERT(fb4_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(e1);
	fb4_free(e2);
	eb_free(p);
	eb_free(q);
	eb_free(r);
	bn_free(k);
	bn_free(n);
	return code;
}

int main(void) {
	int r0 = STS_ERR, r1 = STS_ERR;

	if (core_init() != STS_OK) {
		core_clean();
		return 1;
	}

	fb_param_set_any();

	util_banner("Tests for the PB module", 0);

#ifdef WITH_EB
	r0 = eb_param_set_any_super();
	if (r0 == STS_OK) {
		util_banner("Bilinear pairing (genus 1 curve):\n", 0);

		if (pairing1() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

#ifdef WITH_HB
	r1 = hb_param_set_any_super();
	if (r1 == STS_OK) {
		util_banner("Bilinear pairing (genus 2 curve):\n", 0);

		if (pairing2() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

	if (r0 == STS_ERR && r1 == STS_ERR) {
		THROW(ERR_NO_CURVE);
		core_clean();
		return 0;
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
