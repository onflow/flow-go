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
 * Tests for ternary field arithmetic.
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
	ft_t a;

	ft_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			ft_new(a);
			ft_free(a);
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
	int bits, trits, code = STS_ERR;
	ft_t a, b, c;
	char str[1000];

	ft_null(a);
	ft_null(b);
	ft_null(c);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);

		TEST_BEGIN("comparison is consistent") {
			ft_rand(a);
			ft_rand(b);
			if (ft_cmp(a, b) != CMP_EQ) {
				if (ft_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(ft_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(ft_cmp(b, a) == CMP_GT, end);
				}
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			ft_rand(a);
			ft_rand(b);
			ft_rand(c);
			if (ft_cmp(a, c) != CMP_EQ) {
				ft_copy(c, a);
				TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
			}
			if (ft_cmp(b, c) != CMP_EQ) {
				ft_copy(c, b);
				TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			ft_rand(a);
			ft_neg(b, a);
			if (ft_cmp(a, b) != CMP_EQ) {
				if (ft_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(ft_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(ft_cmp(b, a) == CMP_GT, end);
				}
			}
			ft_neg(b, b);
			TEST_ASSERT(ft_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			ft_rand(a);
			ft_zero(c);
			TEST_ASSERT(ft_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(ft_cmp(c, a) == CMP_LT, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			ft_rand(a);
			ft_rand(b);
			ft_zero(c);
			TEST_ASSERT(ft_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(ft_cmp(b, c) == CMP_GT, end);
			if (ft_cmp(a, b) != CMP_EQ) {
				if (ft_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(ft_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(ft_cmp(b, a) == CMP_GT, end);
				}
			}
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			ft_zero(a);
			TEST_ASSERT(ft_is_zero(a), end);
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("bit setting and getting are consistent") {
			ft_zero(a);
			ft_set_bit(a, bits, 1);
			TEST_ASSERT(ft_get_bit(a, bits) == 1, end);
			ft_set_bit(a, bits, 0);
			TEST_ASSERT(ft_get_bit(a, bits) == 0, end);
			bits = (bits + 1) % FT_BITS;
		}
		TEST_END;

		trits = 0;
		TEST_BEGIN("trit setting and getting are consistent") {
			ft_zero(a);
			ft_set_trit(a, trits, 1);
			TEST_ASSERT(ft_get_trit(a, trits) == 1, end);
			ft_set_bit(a, trits, 0);
			TEST_ASSERT(ft_get_trit(a, trits) == 0, end);
			trits = (trits + 1) % FT_TRITS;
		}
		TEST_END;

		TEST_BEGIN("reading and writing the first digit are consistent") {
			ft_rand(a);
			ft_rand(b);
			ft_set_dig(b, a[0]);
			TEST_ASSERT(a[0] == b[0], end);
		} TEST_END;

		bits = 0;
		TEST_BEGIN("bit setting and testing are consistent") {
			ft_zero(a);
			ft_set_bit(a, bits, 1);
			TEST_ASSERT(ft_test_bit(a, bits), end);
			bits = (bits + 1) % FT_TRITS;
		}
		TEST_END;

		trits = 0;
		TEST_BEGIN("trit setting and testing are consistent") {
			ft_zero(a);
			ft_set_trit(a, trits, 1);
			TEST_ASSERT(ft_test_trit(a, trits), end);
			trits = (trits + 1) % FT_TRITS;
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("bit assignment and counting are consistent") {
			ft_zero(a);
			ft_set_bit(a, bits, 1);
			TEST_ASSERT(ft_bits(a) == bits + 1, end);
			bits = (bits + 1) % FT_BITS;
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("trit assignment and counting are consistent") {
			ft_zero(a);
			ft_set_trit(a, trits, 1);
			TEST_ASSERT(ft_trits(a) == trits + 1, end);
			bits = (bits + 1) % FT_TRITS;
		}
		TEST_END;

		TEST_BEGIN("reading and writing a ternary field element are consistent") {
			ft_rand(a);
			ft_write(str, sizeof(str), a);;
			ft_read(b, str, sizeof(str));
			TEST_ASSERT(ft_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("getting the size of ternary field element is correct") {
			ft_rand(a);
			ft_size(&trits, a);
			trits--;
			TEST_ASSERT(trits == ft_trits(a), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	return code;
}

static int addition(void) {
	int code = STS_ERR;
	ft_t a, b, c, d, e;

	ft_null(a);
	ft_null(b);
	ft_null(c);
	ft_null(d);
	ft_null(e);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);
		ft_new(d);
		ft_new(e);

		TEST_BEGIN("addition is commutative") {
			ft_rand(a);
			ft_rand(b);
			ft_add(d, a, b);
			ft_add(e, b, a);
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			ft_rand(a);
			ft_rand(b);
			ft_rand(c);
			ft_add(d, a, b);
			ft_add(d, d, c);
			ft_add(e, b, c);
			ft_add(e, a, e);
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			ft_rand(a);
			ft_zero(d);
			ft_add(e, a, d);
			TEST_ASSERT(ft_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			ft_rand(a);
			ft_neg(d, a);
			ft_add(e, a, d);
			TEST_ASSERT(ft_is_zero(e), end);
		} TEST_END;

		TEST_BEGIN("addition of the modulo f(z) is correct") {
			ft_rand(a);
			ft_poly_add(d, a);
			ft_add(e, a, ft_poly_get());
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	ft_free(d);
	ft_free(e);
	return code;
}

static int subtraction(void) {
	int code = STS_ERR;
	ft_t a, b, c, d;

	ft_null(a);
	ft_null(b);
	ft_null(c);
	ft_null(d);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);
		ft_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			ft_rand(a);
			ft_rand(b);
			ft_sub(c, a, b);
			ft_sub(d, b, a);
			ft_neg(d, d);
			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			ft_rand(a);
			ft_zero(c);
			ft_sub(d, a, c);
			TEST_ASSERT(ft_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			ft_rand(a);
			ft_sub(c, a, a);
			TEST_ASSERT(ft_is_zero(c), end);
		}
		TEST_END;

		TEST_BEGIN("subtraction of the modulo f(z) is correct") {
			ft_rand(a);
			ft_poly_sub(c, a);
			ft_sub(d, a, ft_poly_get());
			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("subtraction is the inverse of addition") {
			ft_rand(a);
			ft_rand(b);
			ft_add(c, a, b);
			ft_sub(d, c, b);
			TEST_ASSERT(ft_cmp(a, d) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	ft_free(d);
	return code;
}

static int multiplication(void) {
	int code = STS_ERR;
	ft_t a, b, c, d, e, f;

	ft_null(a);
	ft_null(b);
	ft_null(c);
	ft_null(d);
	ft_null(e);
	ft_null(f);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);
		ft_new(d);
		ft_new(e);
		ft_new(f);

		TEST_BEGIN("multiplication is commutative") {
			ft_rand(a);
			ft_rand(b);
			ft_mul(d, a, b);
			ft_mul(e, b, a);
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			ft_rand(a);
			ft_rand(b);
			ft_rand(c);
			ft_mul(d, a, b);
			ft_mul(d, d, c);
			ft_mul(e, b, c);
			ft_mul(e, a, e);
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			ft_rand(a);
			ft_rand(b);
			ft_rand(c);
			ft_add(d, a, b);
			ft_mul(d, c, d);
			ft_mul(e, c, a);
			ft_mul(f, c, b);
			ft_add(e, e, f);
			TEST_ASSERT(ft_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			ft_rand(a);
			ft_zero(d);
			ft_set_bit(d, 0, 1);
			ft_mul(e, a, d);
			TEST_ASSERT(ft_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			ft_rand(a);
			ft_zero(d);
			ft_mul(e, a, d);
			TEST_ASSERT(ft_is_zero(e), end);
		} TEST_END;

#if FT_MUL == BASIC || !defined(STRIP)
		TEST_BEGIN("basic multiplication is correct") {
			ft_rand(a);
			ft_rand(b);
			ft_mul(c, a, b);
			ft_mul_basic(d, a, b);
			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if FT_MUL == LODAH || !defined(STRIP)
		TEST_BEGIN("lopez-dahab multiplication is correct") {
			ft_rand(a);
			ft_rand(b);
			ft_mul(c, a, b);
			ft_mul_lodah(d, a, b);
			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if FT_MUL == INTEG || !defined(STRIP)
		TEST_BEGIN("integrated multiplication is correct") {
			ft_rand(a);
			ft_rand(b);
			ft_mul(c, a, b);
			ft_mul_integ(d, a, b);
			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif
//
//#if FT_MUL == LCOMB || !defined(STRIP)
//		TEST_BEGIN("left-to-right comb multiplication is correct") {
//			ft_rand(a);
//			ft_rand(b);
//			ft_mul(c, a, b);
//			ft_mul_lcomb(d, a, b);
//			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
//		}
//		TEST_END;
//#endif
//
//#if FT_MUL == RCOMB || !defined(STRIP)
//		TEST_BEGIN("right-to-left comb multiplication is correct") {
//			ft_rand(a);
//			ft_rand(b);
//			ft_mul(c, a, b);
//			ft_mul_rcomb(d, a, b);
//			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
//		}
//		TEST_END;
//#endif
//
//#if FT_KARAT > 0 || !defined(STRIP)
//		TEST_BEGIN("karatsuba multiplication is correct") {
//			ft_rand(a);
//			ft_rand(b);
//			ft_mul(c, a, b);
//			ft_mul_karat(d, a, b);
//			TEST_ASSERT(ft_cmp(c, d) == CMP_EQ, end);
//		}
//		TEST_END;
//#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	ft_free(d);
	ft_free(e);
	ft_free(f);
	return code;
}

static int cubing(void) {
	int code = STS_ERR;
	ft_t a, b, c;

	ft_null(a);
	ft_null(b);
	ft_null(c);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);

		TEST_BEGIN("cubing is correct") {
			ft_rand(a);
			ft_mul(b, a, a);
			ft_mul(b, b, a);
			ft_cub(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if FT_CUB == BASIC || !defined(STRIP)
		TEST_BEGIN("basic cubing is correct") {
			ft_rand(a);
			ft_cub(b, a);
			ft_cub_basic(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FT_CUB == TABLE || !defined(STRIP)
		TEST_BEGIN("table cubing is correct") {
			ft_rand(a);
			ft_cub(b, a);
			ft_cub_table(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FT_CUB == INTEG || !defined(STRIP)
		TEST_BEGIN("integrated cubing is correct") {
			ft_rand(a);
			ft_cub(b, a);
			ft_cub_integ(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	return code;
}

static int cube_root(void) {
	int code = STS_ERR;
	ft_t a, b, c;

	ft_null(a);
	ft_null(b);
	ft_null(c);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);

		TEST_BEGIN("cube root extraction is correct") {
			ft_rand(a);
			ft_cub(c, a);
			ft_crt(b, c);
			TEST_ASSERT(ft_cmp(b, a) == CMP_EQ, end);
		} TEST_END;

#if FT_CRT == BASIC || !defined(STRIP)
		TEST_BEGIN("basic cube root extraction is correct") {
			ft_rand(a);
			ft_crt(b, a);
			ft_crt_basic(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FT_CRT == QUICK || !defined(STRIP)
		TEST_BEGIN("fast cube root extraction is correct") {
			ft_rand(a);
			ft_crt(b, a);
			ft_crt_quick(c, a);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		}
		TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	return code;
}

static int shifting(void) {
	int code = STS_ERR;
	ft_t a, b, c;

	ft_null(a);
	ft_null(b);
	ft_null(c);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);

		TEST_BEGIN("shifting by 1 trit is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			ft_lsh(b, a, 1);
			ft_rsh(c, b, 1);
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 trits is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			ft_lsh(b, a, 2);
			ft_rsh(c, b, 2);
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by half digit is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			ft_lsh(b, a, FT_DIGIT / 2);
			ft_rsh(c, b, FT_DIGIT / 2);
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 1 digit is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			ft_lsh(b, a, FT_DIGIT);
			ft_rsh(c, b, FT_DIGIT);
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 digits is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			a[FT_DIGS / 2 - 2] = a[FT_DIGS - 2] = 0;
			ft_lsh(b, a, 2 * FT_DIGIT);
			ft_rsh(c, b, 2 * FT_DIGIT);
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 1 digit and half is consistent") {
			ft_rand(a);
			a[FT_DIGS / 2 - 1] = a[FT_DIGS - 1] = 0;
			a[FT_DIGS / 2 - 2] = a[FT_DIGS - 2] = 0;
			ft_lsh(b, a, FT_DIGIT + FT_DIGIT / 2);
			ft_rsh(c, b, (FT_DIGIT + FT_DIGIT / 2));
			TEST_ASSERT(ft_cmp(c, a) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	return code;
}

static int reduction(void) {
	int code = STS_ERR;
	ft_t a, b, c;
	dv_t t0, t1;

	ft_null(a);
	ft_null(b);
	ft_null(c);
	dv_null(t0);
	dv_null(t1);

	TRY {
		ft_new(a);
		ft_new(b);
		ft_new(c);
		dv_new(t0);
		dv_new(t1);
		dv_zero(t0, 2 * FT_DIGS);
		dv_zero(t1, 2 * FT_DIGS);

		TEST_BEGIN("modular reduction of multiplication result is correct") {
			ft_rand(a);
			/* Test if a * f(z) mod f(z) == 0. */
			ft_mul(b, a, ft_poly_get());
			TEST_ASSERT(ft_is_zero(b) == 1, end);
		} TEST_END;

#if FT_RDC == BASIC || !defined(STRIP)
		TEST_BEGIN("basic modular reduction of multiplication result is correct") {
			ft_rand(a);
			dv_copy(t0, a, FT_DIGS/2);
			dv_copy(t0 + FT_DIGS/2, a, FT_DIGS/2);
			dv_copy(t0 + 3 * FT_DIGS/2, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t0 + 2 * FT_DIGS, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t1, t0, 3 * FT_DIGS);
			ft_rdc_mul(b, t0);
			ft_rdc_mul_basic(c, t1);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FT_RDC == QUICK || !defined(STRIP)
		TEST_BEGIN("fast modular reduction of multiplication result is correct") {
			ft_rand(a);
			dv_copy(t0, a, FT_DIGS/2);
			dv_copy(t0 + FT_DIGS/2, a, FT_DIGS/2);
			dv_copy(t0 + 3 * FT_DIGS/2, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t0 + 2 * FT_DIGS, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t1, t0, 3 * FT_DIGS);
			ft_rdc_mul(b, t0);
			ft_rdc_mul_quick(c, t1);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

		TEST_BEGIN("modular reduction of cubing result is correct") {
			/* Test if f(z)^3 mod f(z) == 0. */
			ft_cub(b, ft_poly_get());
			TEST_ASSERT(ft_is_zero(b) == 1, end);
		} TEST_END;

		dv_zero(t0, 3 * FT_DIGS);
		dv_zero(t1, 3 * FT_DIGS);

#if FT_RDC == BASIC || !defined(STRIP)
		TEST_BEGIN("basic modular reduction of cubing result is correct") {
			ft_rand(a);
			dv_copy(t0, a, FT_DIGS/2);
			dv_copy(t0 + FT_DIGS/2, a, FT_DIGS/2);
			dv_copy(t0 + 3 * FT_DIGS/2, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t0 + 2 * FT_DIGS, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t1, t0, 3 * FT_DIGS);
			ft_rdc_cub(b, t0);
			ft_rdc_cub_basic(c, t1);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FT_RDC == QUICK || !defined(STRIP)
		TEST_BEGIN("fast modular reduction of cubing result is correct") {
			ft_rand(a);
			dv_copy(t0, a, FT_DIGS/2);
			dv_copy(t0 + FT_DIGS/2, a, FT_DIGS/2);
			dv_copy(t0 + 3 * FT_DIGS/2, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t0 + 2 * FT_DIGS, a + FT_DIGS/2, FT_DIGS/2);
			dv_copy(t1, t0, 3 * FT_DIGS);
			ft_rdc_cub(b, t0);
			ft_rdc_cub_quick(c, t1);
			TEST_ASSERT(ft_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ft_free(a);
	ft_free(b);
	ft_free(c);
	dv_free(t0);
	dv_free(t1);
	return code;
}

int main(void) {
	if (core_init() != STS_OK) {
		core_clean();
		return 1;
	}

	util_banner("Tests for the FT module", 0);

	TRY {
		ft_param_set_any();
		ft_param_print();
	} CATCH_ANY {
		core_clean();
		return 0;
	}

	util_banner("Utilities", 1);
	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util() != STS_OK) {
		core_clean();
		return 1;
	}
	util_banner("Arithmetic", 1);

	if (addition() != STS_OK) {
		core_clean();
		return 1;
	}

	if (subtraction() != STS_OK) {
		core_clean();
		return 1;
	}

	if (multiplication() != STS_OK) {
		core_clean();
		return 1;
	}

	if (cubing() != STS_OK) {
		core_clean();
		return 1;
	}

	if (cube_root() != STS_OK) {
		core_clean();
		return 1;
	}

	if (shifting() != STS_OK) {
		core_clean();
		return 1;
	}

	if (reduction() != STS_OK) {
		core_clean();
		return 1;
	}

	util_banner("All tests have passed.\n", 0);

	core_clean();
	return 0;
}
