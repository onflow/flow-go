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
 * Tests for pairings defined over prime elliptic curves.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

static int pairing(void) {
	int code = STS_ERR;
	fp12_t e1, e2;
	ep2_t q, r, s;
	ep_t p;
	bn_t k, n;

	fp12_null(e1);
	fp12_null(e2);
	ep_null(p);
	ep2_null(q);
	ep2_null(r);
	ep2_null(s);
	bn_null(k);
	bn_null(n);

	TRY {
		fp12_new(e1);
		fp12_new(e2);
		ep_new(p);
		ep2_new(q);
		ep2_new(r);
		ep2_new(s);
		bn_new(n);
		bn_new(k);

		ep_curve_get_ord(n);

		TEST_BEGIN("miller addition is consistent") {
			ep_rand(p);
			ep2_rand(q);
			ep2_rand(r);
			ep2_copy(s, r);
			pp_add_k12(e1, r, q, p);
			pp_norm(r, r);
			ep2_add(s, s, q);
			ep2_norm(s, s);
			TEST_ASSERT(ep2_cmp(r, s) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("miller doubling is consistent") {
			ep_rand(p);
			ep2_rand(q);
			ep2_rand(r);
			ep2_copy(s, r);
			pp_dbl_k12(e1, r, q, p);
			pp_norm(r, r);
			ep2_dbl(s, q);
			ep2_norm(s, s);
			TEST_ASSERT(ep2_cmp(r, s) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("pairing is not degenerate") {
			ep_rand(p);
			ep2_rand(q);
			pp_map(e1, p, q);
			fp12_zero(e2);
			fp_set_dig(e2[0][0][0], 1);
			TEST_ASSERT(fp12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("pairing is bilinear") {
			ep_rand(p);
			ep2_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			ep2_mul(r, q, k);
			fp12_zero(e1);
			fp12_zero(e2);
			pp_map(e1, p, r);
			ep_mul(p, p, k);
			pp_map(e2, p, q);
			TEST_ASSERT(fp12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

#if PP_MAP == TATEP || !defined(STRIP)
		TEST_BEGIN("tate pairing is not degenerate") {
			ep_rand(p);
			ep2_rand(q);
			pp_map_tatep(e1, p, q);
			fp12_zero(e2);
			fp_set_dig(e2[0][0][0], 1);
			TEST_ASSERT(fp12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("tate pairing is bilinear") {
			ep_rand(p);
			ep2_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			ep2_mul(r, q, k);
			fp12_zero(e1);
			fp12_zero(e2);
			pp_map_tatep(e1, p, r);
			ep_mul(p, p, k);
			pp_map_tatep(e2, p, q);
			TEST_ASSERT(fp12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif

#if PP_MAP == WEIL || !defined(STRIP)
		TEST_BEGIN("weil pairing is not degenerate") {
			ep_rand(p);
			ep2_rand(q);
			pp_map_weilp(e1, p, q);
			fp12_zero(e2);
			fp_set_dig(e2[0][0][0], 1);
			TEST_ASSERT(fp12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("weil pairing is bilinear") {
			ep_rand(p);
			ep2_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			ep2_mul(r, q, k);
			fp12_zero(e1);
			fp12_zero(e2);
			pp_map_weilp(e1, p, r);
			ep_mul(p, p, k);
			pp_map_weilp(e2, p, q);
			TEST_ASSERT(fp12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif

#if PP_MAP == OATEP || !defined(STRIP)
		TEST_BEGIN("optimal ate pairing is not degenerate") {
			ep_rand(p);
			ep2_rand(q);
			pp_map_oatep(e1, p, q);
			fp12_zero(e2);
			fp_set_dig(e2[0][0][0], 1);
			TEST_ASSERT(fp12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal ate pairing is bilinear") {
			ep_rand(p);
			ep2_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			ep2_mul(r, q, k);
			fp12_zero(e1);
			fp12_zero(e2);
			pp_map_oatep(e1, p, r);
			ep_mul(p, p, k);
			pp_map_oatep(e2, p, q);
			TEST_ASSERT(fp12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(e1);
	fp12_free(e2);
	ep_free(p);
	ep2_free(q);
	ep2_free(r);
	bn_free(n);
	bn_free(k);
	return code;
}

int main(void) {
	core_init();

	util_banner("Tests for the PP module", 0);

	if (ep_param_set_any_pairf() == STS_ERR) {
		THROW(ERR_NO_CURVE);
		core_clean();
		return 0;
	}

	ep_param_print();

	util_banner("Arithmetic", 1);

	if (pairing() != STS_OK) {
		core_clean();
		return 1;
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
