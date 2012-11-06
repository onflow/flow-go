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
 * Tests for arithmetic on binary hyperelliptic curves.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

static int memory(void) {
	err_t e;
	int code = STS_ERR;
	hb_t a;

	hb_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			hb_new(a);
			hb_free(a);
		} TEST_END;
	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
	(void)a;
	code = STS_OK;
  end:
	return code;
}

static int util(void) {
	int code = STS_ERR;
	hb_t a, b, c;

	hb_null(a);
	hb_null(b);
	hb_null(c);

	TRY {
		hb_new(a);
		hb_new(b);
		hb_new(c);

		TEST_BEGIN("copy and comparison are consistent") {
			hb_rand(a);
			hb_rand(b);
			hb_rand(c);
			if (hb_cmp(a, c) != CMP_EQ) {
				hb_copy(c, a);
				TEST_ASSERT(hb_cmp(c, a) == CMP_EQ, end);
			}
			if (hb_cmp(b, c) != CMP_EQ) {
				hb_copy(c, b);
				TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			hb_rand(a);
			hb_rand(b);
			hb_neg(b, a);
			TEST_ASSERT(hb_cmp(a, b) != CMP_EQ, end);
			hb_neg(b, b);
			TEST_ASSERT(hb_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			hb_rand(a);
			hb_set_infty(c);
			TEST_ASSERT(hb_cmp(a, c) != CMP_EQ, end);
			TEST_ASSERT(hb_cmp(c, a) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to infinity and infinity test are consistent") {
			hb_set_infty(a);
			TEST_ASSERT(hb_is_infty(a), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(a);
	hb_free(b);
	hb_free(c);
	return code;
}

static int addition(void) {
	int code = STS_ERR;
	hb_t a, b, c, d, e;

	hb_null(a);
	hb_null(b);
	hb_null(c);
	hb_null(d);
	hb_null(e);

	TRY {
		hb_new(a);
		hb_new(b);
		hb_new(c);
		hb_new(d);
		hb_new(e);

		TEST_BEGIN("divisor class addition is commutative") {
			hb_rand(a);
			hb_rand(b);
			hb_add(d, a, b);
			hb_add(e, b, a);
//			hb_norm(d, d);
//			hb_norm(e, e);
			TEST_ASSERT(hb_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("divisor class addition is associative") {
			hb_rand(a);
			hb_rand(b);
			hb_rand(c);
			hb_add(d, a, b);
			hb_add(d, d, c);
			hb_add(e, b, c);
			hb_add(e, e, a);
//			hb_norm(d, d);
//			hb_norm(e, e);
			TEST_ASSERT(hb_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("divisor class addition has identity") {
			hb_rand(a);
			hb_set_infty(d);
			hb_add(e, a, d);
			TEST_ASSERT(hb_cmp(e, a) == CMP_EQ, end);
			hb_add(e, d, a);
			TEST_ASSERT(hb_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("divisor class addition has inverse") {
			hb_rand(a);
			hb_neg(d, a);
			hb_add(e, a, d);
			TEST_ASSERT(hb_is_infty(e), end);
		} TEST_END;

#if HB_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("divisor class addition in affine coordinates is correct") {
			hb_rand(a);
			hb_rand(b);
			hb_add(d, a, b);
			//hb_norm(d, d);
			hb_add_basic(e, a, b);
			TEST_ASSERT(hb_cmp(e, d) == CMP_EQ, end);
		} TEST_END;
#endif

//#if HB_ADD == PROJC || !defined(STRIP)
//		TEST_BEGIN("divisor class addition in projective coordinates is correct") {
//			hb_rand(a);
//			hb_rand(b);
//			hb_add_projc(a, a, b);
//			hb_rand(b);
//			hb_rand(c);
//			hb_add_projc(b, b, c);
//			/* a and b in projective coordinates. */
//			hb_add_projc(d, a, b);
//			hb_norm(d, d);
//			hb_norm(a, a);
//			hb_norm(b, b);
//			hb_add(e, a, b);
//			hb_norm(e, e);
//			TEST_ASSERT(hb_cmp(e, d) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN("divisor class addition in mixed coordinates (z2 = 1) is correct") {
//			hb_rand(a);
//			hb_rand(b);
//			hb_add_projc(a, a, b);
//			hb_rand(b);
//			/* a and b in projective coordinates. */
//			hb_add_projc(d, a, b);
//			hb_norm(d, d);
//			/* a in affine coordinates. */
//			hb_norm(a, a);
//			hb_add(e, a, b);
//			hb_norm(e, e);
//			TEST_ASSERT(hb_cmp(e, d) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN("divisor class addition in mixed coordinates (z1,z2 = 1) is correct") {
//			hb_rand(a);
//			hb_rand(b);
//			hb_norm(a, a);
//			hb_norm(b, b);
//			/* a and b in affine coordinates. */
//			hb_add(d, a, b);
//			hb_norm(d, d);
//			hb_add_projc(e, a, b);
//			hb_norm(e, e);
//			TEST_ASSERT(hb_cmp(e, d) == CMP_EQ, end);
//		} TEST_END;
//#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(a);
	hb_free(b);
	hb_free(c);
	hb_free(d);
	hb_free(e);
	return code;
}

static int subtraction(void) {
	int code = STS_ERR;
	hb_t a, b, c, d;

	hb_null(a);
	hb_null(b);
	hb_null(c);
	hb_null(d);

	TRY {
		hb_new(a);
		hb_new(b);
		hb_new(c);
		hb_new(d);

		TEST_BEGIN("divisor class subtraction is anti-commutative") {
			hb_rand(a);
			hb_rand(b);
			hb_sub(c, a, b);
			hb_sub(d, b, a);
//			hb_norm(c, c);
//			hb_norm(d, d);
			hb_neg(d, d);
			TEST_ASSERT(hb_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("divisor class subtraction has identity") {
			hb_rand(a);
			hb_set_infty(c);
			hb_sub(d, a, c);
			//hb_norm(d, d);
			TEST_ASSERT(hb_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("divisor class subtraction has inverse") {
			hb_rand(a);
			hb_sub(c, a, a);
			//hb_norm(c, c);
			TEST_ASSERT(hb_is_infty(c), end);
		}
		TEST_END;

#if HB_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("divisor class subtraction in affine coordinates is correct") {
			hb_rand(a);
			hb_rand(b);
			hb_sub(c, a, b);
			//hb_norm(c, c);
			hb_sub_basic(d, a, b);
			TEST_ASSERT(hb_cmp(c, d) == CMP_EQ, end);
		} TEST_END;
#endif

//#if HB_ADD == PROJC || !defined(STRIP)
//		TEST_BEGIN("divisor class subtraction in projective coordinates is correct") {
//			hb_rand(a);
//			hb_rand(b);
//			hb_add_projc(a, a, b);
//			hb_rand(b);
//			hb_rand(c);
//			hb_add_projc(b, b, c);
//			/* a and b in projective coordinates. */
//			hb_sub_projc(c, a, b);
//			hb_norm(c, c);
//			hb_norm(a, a);
//			hb_norm(b, b);
//			hb_sub(d, a, b);
//			hb_norm(d, d);
//			TEST_ASSERT(hb_cmp(c, d) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN("divisor class subtraction in mixed coordinates (z2 = 1) is correct") {
//			hb_rand(a);
//			hb_rand(b);
//			hb_add_projc(a, a, b);
//			hb_rand(b);
//			/* a and b in projective coordinates. */
//			hb_sub_projc(c, a, b);
//			hb_norm(c, c);
//			/* a in affine coordinates. */
//			hb_norm(a, a);
//			hb_sub(d, a, b);
//			hb_norm(d, d);
//			TEST_ASSERT(hb_cmp(c, d) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN
//				("divisor class subtraction in mixed coordinates (z1,z2 = 1) is correct")
//		{
//			hb_rand(a);
//			hb_rand(b);
//			hb_norm(a, a);
//			hb_norm(b, b);
//			/* a and b in affine coordinates. */
//			hb_sub(c, a, b);
//			hb_norm(c, c);
//			hb_sub_projc(d, a, b);
//			hb_norm(d, d);
//			TEST_ASSERT(hb_cmp(c, d) == CMP_EQ, end);
//		} TEST_END;
//#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(a);
	hb_free(b);
	hb_free(c);
	hb_free(d);
	return code;
}

static int doubling(void) {
	int code = STS_ERR;
	hb_t a, b, c;

	hb_null(a);
	hb_null(b);
	hb_null(c);

	TRY {
		hb_new(a);
		hb_new(b);
		hb_new(c);

		TEST_BEGIN("divisor class doubling is correct") {
			hb_rand(a);
			hb_add(b, a, a);
			hb_dbl(c, a);
			//hb_norm(b, b);
			//hb_norm(c, c);
			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if HB_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("divisor class doubling in affine coordinates is correct") {
			hb_rand(a);
			hb_dbl(b, a);
			//hb_norm(b, b);
			hb_dbl_basic(c, a);
			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

//#if HB_ADD == PROJC || !defined(STRIP)
//		TEST_BEGIN("divisor class doubling in projective coordinates is correct") {
//			hb_rand(a);
//			hb_dbl_projc(a, a);
//			/* a in projective coordinates. */
//			hb_dbl_projc(b, a);
//			hb_norm(b, b);
//			hb_norm(a, a);
//			hb_dbl(c, a);
//			hb_norm(c, c);
//			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN("divisor class doubling in mixed coordinates (z1 = 1) is correct") {
//			hb_rand(a);
//			hb_dbl_projc(b, a);
//			hb_norm(b, b);
//			hb_dbl(c, a);
//			hb_norm(c, c);
//			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
//		} TEST_END;
//#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(a);
	hb_free(b);
	hb_free(c);
	return code;
}

static int octupling(void) {
	int code = STS_ERR;
	hb_t a, b, c;

	hb_null(a);
	hb_null(b);
	hb_null(c);

	TRY {
		hb_new(a);
		hb_new(b);
		hb_new(c);

		TEST_BEGIN("divisor class octupling is correct") {
			hb_rand(a);
			hb_oct(b, a);
			hb_dbl(c, a);
			hb_dbl(c, c);
			hb_dbl(c, c);
			//hb_norm(b, b);
			//hb_norm(c, c);
			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if HB_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("divisor class octupling in affine coordinates is correct") {
			hb_rand(a);
			hb_oct(b, a);
			//hb_norm(b, b);
			hb_oct_basic(c, a);
			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

//#if HB_ADD == PROJC || !defined(STRIP)
//		TEST_BEGIN("divisor class octupling in projective coordinates is correct") {
//			hb_rand(a);
//			hb_dbl_projc(a, a);
//			/* a in projective coordinates. */
//			hb_dbl_projc(b, a);
//			hb_norm(b, b);
//			hb_norm(a, a);
//			hb_dbl(c, a);
//			hb_norm(c, c);
//			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
//		} TEST_END;
//
//		TEST_BEGIN("divisor class octupling in mixed coordinates (z1 = 1) is correct") {
//			hb_rand(a);
//			hb_dbl_projc(b, a);
//			hb_norm(b, b);
//			hb_dbl(c, a);
//			hb_norm(c, c);
//			TEST_ASSERT(hb_cmp(b, c) == CMP_EQ, end);
//		} TEST_END;
//#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(a);
	hb_free(b);
	hb_free(c);
	return code;
}

static int multiplication(void) {
	int code = STS_ERR;
	hb_t p, q, r;
	bn_t n, k;

	bn_null(n);
	bn_null(k);
	hb_null(p);
	hb_null(q);
	hb_null(r);

	TRY {
		hb_new(p);
		hb_new(q);
		hb_new(r);
		bn_new(n);
		bn_new(k);

		hb_curve_get_gen(p);
		hb_curve_get_ord(n);

		TEST_BEGIN("generator has the right order") {
			hb_mul(r, p, n);
			TEST_ASSERT(hb_is_infty(r) == 1, end);
		} TEST_END;

#if HB_MUL == BASIC || !defined(STRIP)
		TEST_BEGIN("binary divisor class multiplication is correct") {
			hb_rand(p);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_basic(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

#if HB_MUL == OCTUP || !defined(STRIP)
		TEST_BEGIN("octupling-based divisor class multiplication is correct") {
			hb_rand(p);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_octup(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

#if HB_MUL == LWNAF || !defined(STRIP)
		TEST_BEGIN("left-to-right w-naf divisor class multiplication is correct") {
			hb_rand(p);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_lwnaf(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(p);
	hb_free(q);
	hb_free(r);
	bn_free(n);
	bn_free(k);
	return code;
}

static int fixed(void) {
	int code = STS_ERR;
	hb_t p, q, r;
	hb_t t[HB_TABLE_MAX];
	bn_t n, k;

	bn_null(n);
	bn_null(k);
	hb_null(p);
	hb_null(q);
	hb_null(r);

	for (int i = 0; i < HB_TABLE_MAX; i++) {
		hb_null(t[i]);
	}

	TRY {
		hb_new(p);
		hb_new(q);
		hb_new(r);
		bn_new(n);
		bn_new(k);

		hb_curve_get_gen(p);
		hb_curve_get_ord(n);

		for (int i = 0; i < HB_TABLE; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre(t, p);
			hb_mul_fix(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE; i++) {
			hb_free(t[i]);
		}

#if HB_FIX == BASIC || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_BASIC; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("binary fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_basic(t, p);
			hb_mul_fix_basic(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_BASIC; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == YAOWI || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_YAOWI; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("yao windowing fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_yaowi(t, p);
			hb_mul_fix_yaowi(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_YAOWI; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == NAFWI || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_NAFWI; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("naf windowing fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_nafwi(t, p);
			hb_mul_fix_nafwi(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_NAFWI; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == COMBS || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_COMBS; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("single-table comb fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_combs(t, p);
			hb_mul_fix_combs(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_COMBS; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == COMBD || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_COMBD; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("double-table comb fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_combd(t, p);
			hb_mul_fix_combd(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_COMBD; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == LWNAF || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_LWNAF; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("left-to-right w-naf fixed divisor multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_lwnaf(t, p);
			hb_mul_fix_lwnaf(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_LWNAF; i++) {
			hb_free(t[i]);
		}
#endif

#if HB_FIX == OCTUP || !defined(STRIP)
		for (int i = 0; i < HB_TABLE_OCTUP; i++) {
			hb_new(t[i]);
		}
		TEST_BEGIN("octupling-based fixed divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			hb_mul(q, p, k);
			hb_mul_pre_octup(t, p);
			hb_mul_fix_octup(q, t, k);
			hb_mul(r, p, k);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
		for (int i = 0; i < HB_TABLE_OCTUP; i++) {
			hb_free(t[i]);
		}
#endif
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(p);
	hb_free(q);
	hb_free(r);
	bn_free(n);
	bn_free(k);
	return code;
}

static int simultaneous(void) {
	int code = STS_ERR;
	hb_t p, q, r, s;
	bn_t n, k, l;

	hb_null(p);
	hb_null(q);
	hb_null(r);
	hb_null(s);

	TRY {

		hb_new(p);
		hb_new(q);
		hb_new(r);
		hb_new(s);
		bn_new(n);
		bn_new(k);
		bn_new(l);

		hb_curve_get_gen(p);
		hb_curve_get_ord(n);

		TEST_BEGIN("simultaneous divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul(q, p, k);
			hb_mul(s, q, l);
			hb_mul_sim(r, p, k, q, l);
			hb_add(q, q, s);
			hb_norm(q, q);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;

#if HB_SIM == BASIC || !defined(STRIP)
		TEST_BEGIN("basic simultaneous divisor class multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul_sim(r, p, k, q, l);
			hb_mul_sim_basic(q, p, k, q, l);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

#if HB_SIM == TRICK || !defined(STRIP)
		TEST_BEGIN("shamir's trick for simultaneous multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul_sim(r, p, k, q, l);
			hb_mul_sim_trick(q, p, k, q, l);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

#if HB_SIM == INTER || !defined(STRIP)
		TEST_BEGIN("interleaving for simultaneous multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul_sim(r, p, k, q, l);
			hb_mul_sim_inter(q, p, k, q, l);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

#if HB_SIM == JOINT || !defined(STRIP)
		TEST_BEGIN("jsf for simultaneous multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul_sim(r, p, k, q, l);
			hb_mul_sim_joint(q, p, k, q, l);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif

		TEST_BEGIN("simultaneous multiplication with generator is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			bn_rand(l, BN_POS, bn_bits(n));
			bn_mod(l, l, n);
			hb_mul_sim_gen(r, k, q, l);
			hb_curve_get_gen(s);
			hb_mul_sim(q, s, k, q, l);
			TEST_ASSERT(hb_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	hb_free(p);
	hb_free(q);
	hb_free(r);
	hb_free(s);
	bn_free(n);
	bn_free(k);
	bn_free(l);
	return code;
}

static int test(void) {
	hb_param_print();

	util_banner("Utilities:", 1);

	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util() != STS_OK) {
		return STS_ERR;
	}

	util_banner("Arithmetic:", 1);

	if (addition() != STS_OK) {
		return STS_ERR;
	}

	if (subtraction() != STS_OK) {
		return STS_ERR;
	}

	if (doubling() != STS_OK) {
		return STS_ERR;
	}

	if (octupling() != STS_OK) {
		return STS_ERR;
	}

	if (multiplication() != STS_OK) {
		return STS_ERR;
	}

	if (fixed() != STS_OK) {
		return STS_ERR;
	}

	if (simultaneous() != STS_OK) {
		return STS_ERR;
	}

	return STS_OK;
}

int main(void) {
	int r0;
	core_init();

	util_banner("Tests for the HB module:", 0);

#if defined(HB_SUPER)
	r0 = hb_param_set_any_super();
	if (r0 == STS_OK) {
		if (test() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

	if (r0 == STS_ERR) {
		if (hb_param_set_any() == STS_ERR) {
			THROW(ERR_NO_CURVE);
			core_clean();
			return 0;
		}
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
