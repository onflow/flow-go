/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Tests for the prime elliptic curve arithmetic module.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

void ep_new_impl(ep_t a) {
	ep_new(a);
}

int memory(void) {
	err_t e;
	int code = STS_ERR;
	ep_t a = NULL;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			/* We need another function call for the stack case, so that
			 * the frame for this function does not provoke an overflow. */
			ep_new_impl(a);
			ep_free(a);
		} TEST_END;
	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
	code = STS_OK;
  end:
	return code;
}

int util(void) {
	int code = STS_ERR;
	ep_t a, b, c;

	TRY {
		ep_new(a);
		ep_new(b);
		ep_new(c);

		TEST_BEGIN("comparison is consistent") {
			ep_rand(a);
			ep_rand(b);
			TEST_ASSERT(ep_cmp(a, b) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			ep_rand(a);
			ep_rand(b);
			if (ep_cmp(a, c) != CMP_EQ) {
				ep_copy(c, a);
				TEST_ASSERT(ep_cmp(c, a) == CMP_EQ, end);
			}
			if (ep_cmp(b, c) != CMP_EQ) {
				ep_copy(c, b);
				TEST_ASSERT(ep_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation and comparison are consistent") {
			ep_rand(a);
			ep_neg(b, a);
			TEST_ASSERT(ep_cmp(a, b) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN
				("assignment to random/infinity and comparison are consistent")
		{
			ep_rand(a);
			ep_set_infty(c);
			TEST_ASSERT(ep_cmp(a, c) != CMP_EQ, end);
			TEST_ASSERT(ep_cmp(c, a) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to infinity and infinity test are consistent") {
			ep_set_infty(a);
			TEST_ASSERT(ep_is_infty(a), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep_free(a);
	ep_free(b);
	ep_free(c);
	return code;
}

int addition(void) {
	int code = STS_ERR;

	ep_t a, b, c, d, e;

	TRY {
		ep_new(a);
		ep_new(b);
		ep_new(c);
		ep_new(d);
		ep_new(e);
		TEST_BEGIN("point addition is commutative") {
			ep_rand(a);
			ep_rand(b);
			ep_add(d, a, b);
			ep_add(e, b, a);
			TEST_ASSERT(ep_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition is associative") {
			ep_rand(a);
			ep_rand(b);
			ep_rand(c);
			ep_add(d, a, b);
			ep_add(d, d, c);
			ep_add(e, b, c);
			ep_add(e, e, a);
			ep_norm(d, d);
			ep_norm(e, e);
			TEST_ASSERT(ep_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition has identity") {
			ep_rand(a);
			ep_set_infty(d);
			ep_add(e, a, d);
			TEST_ASSERT(ep_cmp(e, a) == CMP_EQ, end);
			ep_add(e, d, a);
			TEST_ASSERT(ep_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition has inverse") {
			ep_rand(a);
			ep_neg(d, a);
			ep_add(e, a, d);
			TEST_ASSERT(ep_is_infty(e), end);
		} TEST_END;

#if EP_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("point addition in affine coordinates is correct") {
			ep_rand(a);
			ep_rand(b);
			ep_add(d, a, b);
			ep_norm(d, d);
			ep_add_basic(e, a, b);
			TEST_ASSERT(ep_cmp(e, d) == CMP_EQ, end);
		} TEST_END;
#endif

#if 0
#if EP_ADD == PROJC || !defined(STRIP)
		TEST_BEGIN("point addition in projective coordinates is correct") {
			ep_rand(a);
			ep_rand(b);
			ep_add_projc(a, a, b);
			ep_rand(b);
			ep_rand(c);
			ep_add_projc(b, b, c);
			/* a and b in projective coordinates. */
			ep_add_projc(d, a, b);
			ep_norm(d, d);
			ep_norm(a, a);
			ep_norm(b, b);
			ep_add(e, a, b);
			ep_norm(e, e);
			TEST_ASSERT(ep_cmp(e, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition in mixed coordinates (z2 = 1) is correct") {
			ep_rand(a);
			ep_rand(b);
			ep_add_projc(a, a, b);
			ep_rand(b);
			/* a and b in projective coordinates. */
			ep_add_projc(d, a, b);
			ep_norm(d, d);
			/* a in affine coordinates. */
			ep_norm(a, a);
			ep_add(e, a, b);
			ep_norm(e, e);
			TEST_ASSERT(ep_cmp(e, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition in mixed coordinates (z1,z2 = 1) is correct") {
			ep_rand(a);
			ep_rand(b);
			ep_norm(a, a);
			ep_norm(b, b);
			/* a and b in affine coordinates. */
			ep_add(d, a, b);
			ep_norm(d, d);
			ep_add_projc(e, a, b);
			ep_norm(e, e);
			TEST_ASSERT(ep_cmp(e, d) == CMP_EQ, end);
		} TEST_END;
#endif
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep_free(a);
	ep_free(b);
	ep_free(c);
	ep_free(d);
	ep_free(e);
	return code;
}

int subtraction(void) {
	int code = STS_ERR;
	ep_t a, b, c, d;

	TRY {
		ep_new(a);
		ep_new(b);
		ep_new(c);
		ep_new(d);

		TEST_BEGIN("point subtraction is anti-commutative") {
			ep_rand(a);
			ep_rand(b);
			ep_sub(c, a, b);
			ep_sub(d, b, a);
			ep_norm(c, c);
			ep_norm(d, d);
			ep_neg(d, d);
			TEST_ASSERT(ep_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("point subtraction has identity") {
			ep_rand(a);
			ep_set_infty(c);
			ep_sub(d, a, c);
			ep_norm(d, d);
			TEST_ASSERT(ep_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("point subtraction has inverse") {
			ep_rand(a);
			ep_sub(c, a, a);
			ep_norm(c, c);
			TEST_ASSERT(ep_is_infty(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep_free(a);
	ep_free(b);
	ep_free(c);
	ep_free(d);
	return code;
}

int doubling(void) {
	int code = STS_ERR;
	ep_t a, b, c;

	TRY {
		ep_new(a);
		ep_new(b);
		ep_new(c);

		TEST_BEGIN("point doubling is correct") {
			ep_rand(a);
			ep_add(b, a, a);
			ep_norm(b, b);
			ep_dbl(c, a);
			ep_norm(c, c);
			TEST_ASSERT(ep_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if EP_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("point doubling in affine coordinates is correct") {
			ep_rand(a);
			ep_dbl(b, a);
			ep_norm(b, b);
			ep_dbl_basic(c, a);
			TEST_ASSERT(ep_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if 0
#if EP_ADD == PROJC || !defined(STRIP)
		TEST_BEGIN("point doubling in projective coordinates is correct") {
			ep_rand(a);
			ep_dbl_projc(a, a);
			/* a in projective coordinates. */
			ep_dbl_projc(b, a);
			ep_norm(b, b);
			ep_norm(a, a);
			ep_dbl(c, a);
			ep_norm(c, c);
			TEST_ASSERT(ep_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point doubling in mixed coordinates (z1 = 1) is correct") {
			ep_rand(a);
			ep_dbl_projc(b, a);
			ep_norm(b, b);
			ep_dbl(c, a);
			ep_norm(c, c);
			TEST_ASSERT(ep_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep_free(a);
	ep_free(b);
	ep_free(c);
	return code;
}

int multiplication(void) {
	int code = STS_ERR;

	ep_t p;
	ep_t q;
	ep_t r;
	bn_t n;
	bn_t k;

	TRY {
		ep_new(q);
		ep_new(r);
		bn_new(k);

		p = ep_curve_get_gen();
		n = ep_curve_get_ord();

		TEST_BEGIN("generator has the right order") {
			ep_mul(r, p, n);
			TEST_ASSERT(ep_is_infty(r) == 1, end);
		} TEST_END;

#if EP_MUL == BASIC || !defined(STRIP)
		TEST_BEGIN("binary point multiplication is correct") {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod_basic(k, k, n);
			ep_mul(q, p, k);
			ep_mul_basic(r, p, k);
			TEST_ASSERT(ep_cmp(q, r) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep_free(q);
	ep_free(r);
	bn_free(k);
	return code;
}

int test(void) {
	if (util() != STS_OK) {
		core_clean();
		return STS_ERR;
	}

	if (addition() != STS_OK) {
		core_clean();
		return STS_ERR;
	}

	if (subtraction() != STS_OK) {
		core_clean();
		return STS_ERR;
	}

	if (doubling() != STS_OK) {
		core_clean();
		return STS_ERR;
	}

	if (multiplication() != STS_OK) {
		core_clean();
		return STS_ERR;
	}

	return STS_OK;
}

int main(void) {
	core_init();

	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}
#if defined(EP_STAND) && defined(EP_ORDIN) && FP_PRIME == 256
	ep_param_set(RATE_P256);
	util_print("Curve RATE-P256:\n");

	if (test() != STS_OK) {
		core_clean();
		return 1;
	}
#endif

	core_clean();
	return 0;
}
