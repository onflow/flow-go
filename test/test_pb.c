/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Tests for the pairings over binary elliptic curves module.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

static int memory2(void) {
	err_t e;
	int code = STS_ERR;
	fb2_t a;

	fb2_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fb2_new(a);
			fb2_free(a);
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

static int util2(void) {
	int code = STS_ERR;
	fb2_t a, b, c;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);

		TEST_BEGIN("comparison is consistent") {
			fb2_rand(a);
			fb2_rand(b);
			if (fb2_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb2_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_rand(c);
			if (fb2_cmp(a, c) != CMP_EQ) {
				fb2_copy(c, a);
				TEST_ASSERT(fb2_cmp(c, a) == CMP_EQ, end);
			}
			if (fb2_cmp(b, c) != CMP_EQ) {
				fb2_copy(c, b);
				TEST_ASSERT(fb2_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fb2_rand(a);
			fb2_neg(b, a);
			if (fb2_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb2_cmp(b, a) == CMP_NE, end);
			}
			fb2_neg(b, b);
			TEST_ASSERT(fb2_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fb2_rand(a);
			fb2_zero(c);
			TEST_ASSERT(fb2_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fb2_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fb2_rand(a);
			fb2_zero(c);
			TEST_ASSERT(fb2_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fb2_zero(a);
			TEST_ASSERT(fb2_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	return code;
}

static int addition2(void) {
	int code = STS_ERR;
	fb2_t a, b, c, d, e;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);
	fb2_null(d);
	fb2_null(e);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);
		fb2_new(d);
		fb2_new(e);

		TEST_BEGIN("addition is commutative") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_add(d, a, b);
			fb2_add(e, b, a);
			TEST_ASSERT(fb2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_rand(c);
			fb2_add(d, a, b);
			fb2_add(d, d, c);
			fb2_add(e, b, c);
			fb2_add(e, a, e);
			TEST_ASSERT(fb2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fb2_rand(a);
			fb2_zero(d);
			fb2_add(e, a, d);
			TEST_ASSERT(fb2_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fb2_rand(a);
			fb2_neg(d, a);
			fb2_add(e, a, d);
			TEST_ASSERT(fb2_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	fb2_free(d);
	fb2_free(e);
	return code;
}

static int subtraction2(void) {
	int code = STS_ERR;
	fb2_t a, b, c, d;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);
	fb2_null(d);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);
		fb2_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_sub(c, a, b);
			fb2_sub(d, b, a);
			fb2_neg(d, d);
			TEST_ASSERT(fb2_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fb2_rand(a);
			fb2_zero(c);
			fb2_sub(d, a, c);
			TEST_ASSERT(fb2_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fb2_rand(a);
			fb2_sub(c, a, a);
			TEST_ASSERT(fb2_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	fb2_free(d);
	return code;
}

static int multiplication2(void) {
	int code = STS_ERR;
	fb2_t a, b, c, d, e, f;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);
	fb2_null(d);
	fb2_null(e);
	fb2_null(f);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);
		fb2_new(d);
		fb2_new(e);
		fb2_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_mul(d, a, b);
			fb2_mul(e, b, a);
			TEST_ASSERT(fb2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_rand(c);
			fb2_mul(d, a, b);
			fb2_mul(d, d, c);
			fb2_mul(e, b, c);
			fb2_mul(e, a, e);
			TEST_ASSERT(fb2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fb2_rand(a);
			fb2_rand(b);
			fb2_rand(c);
			fb2_add(d, a, b);
			fb2_mul(d, c, d);
			fb2_mul(e, c, a);
			fb2_mul(f, c, b);
			fb2_add(e, e, f);
			TEST_ASSERT(fb2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fb2_zero(d);
			fb_set_bit(d[0], 0, 1);
			fb2_rand(a);
			fb2_mul(e, a, d);
			TEST_ASSERT(fb2_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fb2_zero(d);
			fb2_rand(a);
			fb2_mul(e, a, d);
			TEST_ASSERT(fb2_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	fb2_free(d);
	fb2_free(e);
	fb2_free(f);
	return code;
}

static int squaring2(void) {
	int code = STS_ERR;
	fb2_t a, b, c;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);

		TEST_BEGIN("squaring is correct") {
			fb2_rand(a);
			fb2_mul(b, a, a);
			fb2_sqr(c, a);
			TEST_ASSERT(fb2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	return code;
}

static int inversion2(void) {
	int code = STS_ERR;
	fb2_t a, b, c;

	fb2_null(a);
	fb2_null(b);
	fb2_null(c);

	TRY {
		fb2_new(a);
		fb2_new(b);
		fb2_new(c);

		TEST_BEGIN("inversion is correct") {
			fb2_rand(a);
			fb2_inv(b, a);
			fb2_mul(c, a, b);
			TEST_ASSERT(fb_cmp_dig(c[0], 1) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb2_free(a);
	fb2_free(b);
	fb2_free(c);
	return code;
}

static int memory4(void) {
	err_t e;
	int code = STS_ERR;
	fb4_t a;

	fb4_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fb4_new(a);
			fb4_free(a);
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

static int util4(void) {
	int code = STS_ERR;
	fb4_t a, b, c;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);

		TEST_BEGIN("comparison is consistent") {
			fb4_rand(a);
			fb4_rand(b);
			if (fb4_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb4_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_rand(c);
			if (fb4_cmp(a, c) != CMP_EQ) {
				fb4_copy(c, a);
				TEST_ASSERT(fb4_cmp(c, a) == CMP_EQ, end);
			}
			if (fb4_cmp(b, c) != CMP_EQ) {
				fb4_copy(c, b);
				TEST_ASSERT(fb4_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fb4_rand(a);
			fb4_neg(b, a);
			if (fb4_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb4_cmp(b, a) == CMP_NE, end);
			}
			fb4_neg(b, b);
			TEST_ASSERT(fb4_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fb4_rand(a);
			fb4_zero(c);
			TEST_ASSERT(fb4_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fb4_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fb4_rand(a);
			fb4_zero(c);
			TEST_ASSERT(fb4_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fb4_zero(a);
			TEST_ASSERT(fb4_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	return code;
}

static int addition4(void) {
	int code = STS_ERR;
	fb4_t a, b, c, d, e;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);
	fb4_null(d);
	fb4_null(e);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);
		fb4_new(d);
		fb4_new(e);

		TEST_BEGIN("addition is commutative") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_add(d, a, b);
			fb4_add(e, b, a);
			TEST_ASSERT(fb4_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_rand(c);
			fb4_add(d, a, b);
			fb4_add(d, d, c);
			fb4_add(e, b, c);
			fb4_add(e, a, e);
			TEST_ASSERT(fb4_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fb4_rand(a);
			fb4_zero(d);
			fb4_add(e, a, d);
			TEST_ASSERT(fb4_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fb4_rand(a);
			fb4_neg(d, a);
			fb4_add(e, a, d);
			TEST_ASSERT(fb4_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	fb4_free(d);
	fb4_free(e);
	return code;
}

static int subtraction4(void) {
	int code = STS_ERR;
	fb4_t a, b, c, d;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);
	fb4_null(d);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);
		fb4_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_sub(c, a, b);
			fb4_sub(d, b, a);
			fb4_neg(d, d);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fb4_rand(a);
			fb4_zero(c);
			fb4_sub(d, a, c);
			TEST_ASSERT(fb4_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fb4_rand(a);
			fb4_sub(c, a, a);
			TEST_ASSERT(fb4_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	fb4_free(d);
	return code;
}

static int multiplication4(void) {
	int code = STS_ERR;
	fb4_t a, b, c, d;
	fb_t beta;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);
	fb4_null(d);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);
		fb4_new(d);
		fb_new(beta);

		TEST_BEGIN("multiplication is commutative") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_mul(c, a, b);
			fb4_mul(d, b, a);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_rand(c);
			fb4_mul(d, a, b);
			fb4_mul(d, d, c);
			fb4_mul(c, b, c);
			fb4_mul(c, a, c);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fb4_rand(a);
			fb4_rand(b);
			fb4_rand(c);
			fb4_add(d, a, b);
			fb4_mul(d, c, d);
			fb4_mul(a, c, a);
			fb4_mul(b, c, b);
			fb4_add(c, a, b);
			TEST_ASSERT(fb4_cmp(d, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("dense x dense multiplication is correct") {
			fb_zero(beta);
			fb_set_bit(beta, 3, 1);
			fb_set_bit(beta, 5, 1);
			fb_set_bit(beta, 6, 1);
			fb_set_bit(beta, 8, 1);
			fb4_rand(a);
			fb4_rand(b);
			fb4_mul(c, a, b);
			fb_mul(c[0], c[0], beta);
			fb_mul(c[1], c[1], beta);
			fb_mul(c[2], c[2], beta);
			fb_mul(c[3], c[3], beta);
			fb_mul(c[0], c[0], beta);
			fb_mul(c[1], c[1], beta);
			fb_mul(c[2], c[2], beta);
			fb_mul(c[3], c[3], beta);
			fb4_mul_dxd(d, a, b);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("dense x sparse multiplication is correct") {
			fb4_rand(a);
			fb4_rand(b);
			fb_zero(b[2]);
			fb_set_bit(b[2], 0, 1);
			fb_zero(b[3]);
			fb4_mul(c, a, b);
			fb4_mul_dxs(d, a, b);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("sparse x sparse multiplication is correct") {
			fb4_rand(a);
			fb_zero(a[2]);
			fb_set_bit(a[2], 0, 1);
			fb_zero(a[3]);
			fb4_rand(b);
			fb_zero(b[2]);
			fb_set_bit(b[2], 0, 1);
			fb_zero(b[3]);
			fb4_mul(c, a, b);
			fb4_mul_sxs(d, a, b);
			TEST_ASSERT(fb4_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fb4_zero(b);
			fb4_rand(a);
			fb_set_bit(b[0], 0, 1);
			fb4_mul(c, a, b);
			TEST_ASSERT(fb4_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fb4_zero(b);
			fb4_rand(a);
			fb4_mul(c, a, b);
			TEST_ASSERT(fb4_is_zero(c), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	fb4_free(d);
	fb_free(beta);
	return code;
}

static int squaring4(void) {
	int code = STS_ERR;
	fb4_t a, b, c;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);

		TEST_BEGIN("squaring is correct") {
			fb4_rand(a);
			fb4_mul(b, a, a);
			fb4_sqr(c, a);
			TEST_ASSERT(fb4_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	return code;
}

static int inversion4(void) {
	int code = STS_ERR;
	fb4_t a, b, c;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);

		TEST_BEGIN("inversion is correct") {
			fb4_rand(a);
			fb4_inv(b, a);
			fb4_mul(c, a, b);
			TEST_ASSERT(fb_cmp_dig(c[0], 1) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	return code;
}

static int exponentiation4(void) {
	int code = STS_ERR;
	fb4_t a, b, c;
	bn_t d;

	fb4_null(a);
	fb4_null(b);
	fb4_null(c);
	bn_null(d);

	TRY {
		fb4_new(a);
		fb4_new(b);
		fb4_new(c);
		bn_new(d);

		TEST_BEGIN("exponentiation to 2^m is correct") {
			fb4_rand(a);
			fb4_frb(b, a);
			fb4_copy(c, a);
			for (int j = 0; j < FB_BITS; j++) {
				fb4_sqr(c, c);
			}
			TEST_ASSERT(fb4_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("frobenius and exponentiation are consistent") {
			fb4_rand(a);
			fb4_frb(b, a);
			bn_set_2b(d, FB_BITS);
			fb4_exp(c, a, d);
			TEST_ASSERT(fb4_cmp(c, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb4_free(a);
	fb4_free(b);
	fb4_free(c);
	bn_free(d);
	return code;
}

static int memory6(void) {
	err_t e;
	int code = STS_ERR;
	fb6_t a;

	fb6_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fb6_new(a);
			fb6_free(a);
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

static int util6(void) {
	int code = STS_ERR;
	fb6_t a, b, c;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);

		TEST_BEGIN("comparison is consistent") {
			fb6_rand(a);
			fb6_rand(b);
			if (fb6_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb6_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_rand(c);
			if (fb6_cmp(a, c) != CMP_EQ) {
				fb6_copy(c, a);
				TEST_ASSERT(fb6_cmp(c, a) == CMP_EQ, end);
			}
			if (fb6_cmp(b, c) != CMP_EQ) {
				fb6_copy(c, b);
				TEST_ASSERT(fb6_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fb6_rand(a);
			fb6_neg(b, a);
			if (fb6_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb6_cmp(b, a) == CMP_NE, end);
			}
			fb6_neg(b, b);
			TEST_ASSERT(fb6_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fb6_rand(a);
			fb6_zero(c);
			TEST_ASSERT(fb6_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fb6_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fb6_rand(a);
			fb6_zero(c);
			TEST_ASSERT(fb6_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fb6_zero(a);
			TEST_ASSERT(fb6_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	return code;
}

static int addition6(void) {
	int code = STS_ERR;
	fb6_t a, b, c, d, e;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);
	fb6_null(d);
	fb6_null(e);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);
		fb6_new(d);
		fb6_new(e);

		TEST_BEGIN("addition is commutative") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_add(d, a, b);
			fb6_add(e, b, a);
			TEST_ASSERT(fb6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_rand(c);
			fb6_add(d, a, b);
			fb6_add(d, d, c);
			fb6_add(e, b, c);
			fb6_add(e, a, e);
			TEST_ASSERT(fb6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fb6_rand(a);
			fb6_zero(d);
			fb6_add(e, a, d);
			TEST_ASSERT(fb6_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fb6_rand(a);
			fb6_neg(d, a);
			fb6_add(e, a, d);
			TEST_ASSERT(fb6_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	fb6_free(d);
	fb6_free(e);
	return code;
}

static int subtraction6(void) {
	int code = STS_ERR;
	fb6_t a, b, c, d;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);
	fb6_null(d);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);
		fb6_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_sub(c, a, b);
			fb6_sub(d, b, a);
			fb6_neg(d, d);
			TEST_ASSERT(fb6_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fb6_rand(a);
			fb6_zero(c);
			fb6_sub(d, a, c);
			TEST_ASSERT(fb6_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fb6_rand(a);
			fb6_sub(c, a, a);
			TEST_ASSERT(fb6_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	fb6_free(d);
	return code;
}

static int multiplication6(void) {
	int code = STS_ERR;
	fb6_t a, b, c, d, e, f;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);
	fb6_null(d);
	fb6_null(e);
	fb6_null(f);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);
		fb6_new(d);
		fb6_new(e);
		fb6_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_mul(d, a, b);
			fb6_mul(e, b, a);
			TEST_ASSERT(fb6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_rand(c);
			fb6_mul(d, a, b);
			fb6_mul(d, d, c);
			fb6_mul(e, b, c);
			fb6_mul(e, a, e);
			TEST_ASSERT(fb6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fb6_rand(a);
			fb6_rand(b);
			fb6_rand(c);
			fb6_add(d, a, b);
			fb6_mul(d, c, d);
			fb6_mul(e, c, a);
			fb6_mul(f, c, b);
			fb6_add(e, e, f);
			TEST_ASSERT(fb6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fb6_zero(d);
			fb_set_bit(d[0], 0, 1);
			fb6_mul(e, a, d);
			TEST_ASSERT(fb6_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fb6_zero(d);
			fb6_mul(e, a, d);
			TEST_ASSERT(fb6_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	fb6_free(d);
	fb6_free(e);
	fb6_free(f);
	return code;
}

static int squaring6(void) {
	int code = STS_ERR;
	fb6_t a, b, c;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);

		TEST_BEGIN("squaring is correct") {
			fb6_rand(a);
			fb6_mul(b, a, a);
			fb6_sqr(c, a);
			TEST_ASSERT(fb6_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	return code;
}

static int inversion6(void) {
	int code = STS_ERR;
	fb6_t a, b, c;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);

		TEST_BEGIN("inversion is correct") {
			fb6_rand(a);
			fb6_inv(b, a);
			fb6_mul(c, a, b);
			TEST_ASSERT(fb_cmp_dig(c[0], 1) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	return code;
}

static int exponentiation6(void) {
	int code = STS_ERR;
	fb6_t a, b, c;

	fb6_null(a);
	fb6_null(b);
	fb6_null(c);

	TRY {
		fb6_new(a);
		fb6_new(b);
		fb6_new(c);

		TEST_BEGIN("frobenius action is correct") {
			fb6_rand(a);
			fb6_copy(b, a);
			fb6_frb(b, b);
			fb6_copy(c, a);
			for (int j = 0; j < FB_BITS; j++) {
				fb6_sqr(c, c);
			}
			TEST_ASSERT(fb6_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb6_free(a);
	fb6_free(b);
	fb6_free(c);
	return code;
}

static int memory12(void) {
	err_t e;
	int code = STS_ERR;
	fb12_t a;

	fb12_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fb12_new(a);
			fb12_free(a);
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

static int util12(void) {
	int code = STS_ERR;
	fb12_t a, b, c;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);

		TEST_BEGIN("comparison is consistent") {
			fb12_rand(a);
			fb12_rand(b);
			if (fb12_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb12_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_rand(c);
			if (fb12_cmp(a, c) != CMP_EQ) {
				fb12_copy(c, a);
				TEST_ASSERT(fb12_cmp(c, a) == CMP_EQ, end);
			}
			if (fb12_cmp(b, c) != CMP_EQ) {
				fb12_copy(c, b);
				TEST_ASSERT(fb12_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fb12_rand(a);
			fb12_neg(b, a);
			if (fb12_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fb12_cmp(b, a) == CMP_NE, end);
			}
			fb12_neg(b, b);
			TEST_ASSERT(fb12_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fb12_rand(a);
			fb12_zero(c);
			TEST_ASSERT(fb12_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fb12_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fb12_rand(a);
			fb12_zero(c);
			TEST_ASSERT(fb12_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fb12_zero(a);
			TEST_ASSERT(fb12_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	return code;
}

static int addition12(void) {
	int code = STS_ERR;
	fb12_t a, b, c, d, e;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);
	fb12_null(d);
	fb12_null(e);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);
		fb12_new(d);
		fb12_new(e);

		TEST_BEGIN("addition is commutative") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_add(d, a, b);
			fb12_add(e, b, a);
			TEST_ASSERT(fb12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_rand(c);
			fb12_add(d, a, b);
			fb12_add(d, d, c);
			fb12_add(e, b, c);
			fb12_add(e, a, e);
			TEST_ASSERT(fb12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fb12_rand(a);
			fb12_zero(d);
			fb12_add(e, a, d);
			TEST_ASSERT(fb12_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fb12_rand(a);
			fb12_neg(d, a);
			fb12_add(e, a, d);
			TEST_ASSERT(fb12_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	fb12_free(d);
	fb12_free(e);
	return code;
}

static int subtraction12(void) {
	int code = STS_ERR;
	fb12_t a, b, c, d;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);
	fb12_null(d);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);
		fb12_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_sub(c, a, b);
			fb12_sub(d, b, a);
			fb12_neg(d, d);
			TEST_ASSERT(fb12_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fb12_rand(a);
			fb12_zero(c);
			fb12_sub(d, a, c);
			TEST_ASSERT(fb12_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fb12_rand(a);
			fb12_sub(c, a, a);
			TEST_ASSERT(fb12_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	fb12_free(d);
	return code;
}

static int multiplication12(void) {
	int code = STS_ERR;
	fb12_t a, b, c, d, e, f;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);
	fb12_null(d);
	fb12_null(e);
	fb12_null(f);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);
		fb12_new(d);
		fb12_new(e);
		fb12_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_mul(d, a, b);
			fb12_mul(e, b, a);
			TEST_ASSERT(fb12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_rand(c);
			fb12_mul(d, a, b);
			fb12_mul(d, d, c);
			fb12_mul(e, b, c);
			fb12_mul(e, a, e);
			TEST_ASSERT(fb12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fb12_rand(a);
			fb12_rand(b);
			fb12_rand(c);
			fb12_add(d, a, b);
			fb12_mul(d, c, d);
			fb12_mul(e, c, a);
			fb12_mul(f, c, b);
			fb12_add(e, e, f);
			TEST_ASSERT(fb12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fb12_zero(d);
			fb_set_bit(d[0][0], 0, 1);
			fb12_mul(e, a, d);
			TEST_ASSERT(fb12_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fb12_zero(d);
			fb12_mul(e, a, d);
			TEST_ASSERT(fb12_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	fb12_free(d);
	fb12_free(e);
	fb12_free(f);
	return code;
}

static int squaring12(void) {
	int code = STS_ERR;
	fb12_t a, b, c;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);

		TEST_BEGIN("squaring is correct") {
			fb12_rand(a);
			fb12_mul(b, a, a);
			fb12_sqr(c, a);
			TEST_ASSERT(fb12_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	return code;
}

static int inversion12(void) {
	int code = STS_ERR;
	fb12_t a, b, c;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);

		TEST_BEGIN("inversion is correct") {
			fb12_rand(a);
			fb12_inv(b, a);
			fb12_mul(c, a, b);
			//TEST_ASSERT(fb_cmp_dig(c[0][0], 1) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	return code;
}

static int exponentiation12(void) {
	int code = STS_ERR;
	fb12_t a, b, c;

	fb12_null(a);
	fb12_null(b);
	fb12_null(c);

	TRY {
		fb12_new(a);
		fb12_new(b);
		fb12_new(c);

		TEST_BEGIN("frobenius action is correct") {
			fb12_rand(a);
			fb12_frb(b, a);
			fb12_copy(c, a);
			for (int j = 0; j < FB_BITS; j++) {
				fb12_sqr(c, c);
			}
			//TEST_ASSERT(fb12_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(a);
	fb12_free(b);
	fb12_free(c);
	return code;
}

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

		TEST_BEGIN("etat pairing is non-degenerate") {
			eb_rand(p);
			eb_rand(q);
			pb_map_etat1(e1, p, q);
			fb4_zero(e2);
			fb_set_dig(e2[0], 1);
			TEST_ASSERT(fb4_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing is bilinear") {
			eb_rand(p);
			eb_rand(q);
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
			eb_mul(r, q, k);
			fb4_zero(e1);
			fb4_zero(e2);
			pb_map_etat1(e1, p, r);
			TEST_ASSERT(!bn_is_zero(n), end);
			eb_mul(r, p, k);
			pb_map_etat1(e2, r, q);
			TEST_ASSERT(fb4_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

#if PB_MAP == ETATS || !defined(STRIP)
		TEST_BEGIN("etat pairing with square roots is correct") {
			eb_rand(p);
			eb_rand(q);
			pb_map_etat1(e1, p, q);
			pb_map_etats(e2, p, q);
			TEST_ASSERT(fb4_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
#endif

#if PB_MAP == ETATN || !defined(STRIP)
		TEST_BEGIN("etat pairing without square roots is correct") {
			eb_rand(p);
			eb_rand(q);
			pb_map_etat1(e1, p, q);
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

#ifdef WITH_HB

static int pairing2(void) {
	int code = STS_ERR;
	fb12_t e1, e2;
	hb_t p, q, r;
	bn_t k, n;

	fb12_null(e1);
	fb12_null(e2);
	hb_null(p);
	hb_null(q);
	hb_null(r);
	bn_null(k);
	bn_null(n);

	TRY {
		fb12_new(e1);
		fb12_new(e2);
		hb_new(p);
		hb_new(q);
		hb_new(r);
		bn_new(k);
		bn_new(n);

		hb_curve_get_ord(n);

		TEST_BEGIN("etat pairing with (deg x deg) divisors is non-degenerate") {
			fb12_zero(e1);
			hb_rand_deg(p);
			hb_rand_deg(q);
			pb_map_etat2(e1, p, q);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (deg x deg) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_deg(p);
			hb_rand_deg(q);
			hb_oct(r, q);
			pb_map_etat2(e1, p, r);
			hb_oct(r, p);
			pb_map_etat2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (deg x type-a non) divisors is non-degenerate") {
			fb12_zero(e1);
			hb_rand_deg(p);
			hb_rand_non(q, 0);
			pb_map_etat2(e1, p, q);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (deg x type-a non) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_deg(p);
			hb_rand_non(q, 0);
			hb_oct(r, q);
			pb_map_etat2(e1, p, r);
			hb_oct(r, p);
			pb_map_etat2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-a non x deg) divisors is non-degenerate") {
			fb12_zero(e1);
			hb_rand_non(p, 0);
			hb_rand_deg(q);
			pb_map_etat2(e1, p, q);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-a non x deg) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_non(p, 0);
			hb_rand_deg(q);
			hb_oct(r, q);
			pb_map_etat2(e1, p, r);
			hb_oct(r, p);
			pb_map_etat2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-a x type-a) divisors is non-degenerate") {
			fb12_zero(e1);
			hb_rand_non(p, 0);
			hb_rand_non(q, 0);
			pb_map_etat2(e1, p, q);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-a x type-a) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_non(p, 0);
			hb_rand_non(q, 0);
			hb_oct(r, q);
			pb_map_etat2(e1, p, r);
			hb_oct(r, p);
			pb_map_etat2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-b x type-b) divisors is non-degenerate") {
			fb12_zero(e1);
			hb_rand_non(p, 1);
			hb_rand_non(q, 1);
			pb_map_etat2(e1, p, q);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("etat pairing with (type-b x type-b) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_non(p, 1);
			hb_rand_non(q, 1);
			hb_oct(r, q);
			pb_map_etat2(e1, p, r);
			hb_oct(r, p);
			pb_map_etat2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		hb_curve_get_ord(n);

		TEST_BEGIN("optimal eta pairing with (deg x deg) divisors is non-degenerate") {
			fb12_zero(e1);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			hb_rand_deg(p);
			hb_rand_deg(q);
			pb_map_oeta2(e1, p, q);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal eta pairing with (deg x deg) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_deg(p);
			hb_rand_deg(q);
			hb_oct(r, q);
			pb_map_oeta2(e1, p, r);
			hb_oct(r, p);
			pb_map_oeta2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal eta pairing with (gen x deg) divisors is non-degenerate") {
			fb12_zero(e1);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			hb_rand_non(p, 0);
			hb_rand_deg(q);
			pb_map_oeta2(e1, p, q);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal eta pairing with (gen x deg) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_non(p, 0);
			hb_rand_deg(q);
			hb_oct(r, q);
			pb_map_oeta2(e1, p, r);
			hb_oct(r, p);
			pb_map_oeta2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal eta pairing with (gen x gen) divisors is non-degenerate") {
			fb12_zero(e1);
			fb12_zero(e2);
			fb_set_dig(e2[0][0], 1);
			hb_rand_non(p, 0);
			hb_rand_non(q, 0);
			pb_map_oeta2(e1, p, q);
			TEST_ASSERT(fb12_cmp(e1, e2) != CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("optimal eta pairing with (gen x gen) divisors is bilinear") {
			fb12_zero(e1);
			fb12_zero(e2);
			hb_rand_non(p, 0);
			hb_rand_non(q, 0);
			hb_oct(r, q);
			pb_map_oeta2(e1, p, r);
			hb_oct(r, p);
			pb_map_oeta2(e2, r, q);
			TEST_ASSERT(fb12_cmp(e1, e2) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fb12_free(e1);
	fb12_free(e2);
	eb_free(p);
	eb_free(q);
	eb_free(r);
	bn_free(k);
	bn_free(n);
	return code;
}

#endif

int main(void) {
	int r0, r1;

	core_init();

	fb_param_set_any();

	util_print_banner("Tests for the PB module", 0);

	util_print_banner("Quadratic extension:", 0);
	util_print_banner("Utilities:", 1);

	if (memory2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util2() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Arithmetic:", 1);

	if (addition2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (subtraction2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (multiplication2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (squaring2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (inversion2() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Quartic extension:", 0);
	util_print_banner("Utilities", 1);

	if (memory4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util4() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Arithmetic:", 1);

	if (addition4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (subtraction4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (multiplication4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (squaring4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (inversion4() != STS_OK) {
		core_clean();
		return 1;
	}

	if (exponentiation4() != STS_OK) {
		core_clean();
		return 1;
	}

#ifdef WITH_HB

	util_print_banner("Sextic extension:", 0);
	util_print_banner("Utilities", 1);

	if (memory6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util6() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Arithmetic:", 1);

	if (addition6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (subtraction6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (multiplication6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (squaring6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (inversion6() != STS_OK) {
		core_clean();
		return 1;
	}

	if (exponentiation6() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Dodecic extension:", 0);
	util_print_banner("Utilities", 1);

	if (memory12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util12() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Arithmetic:", 1);

	if (addition12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (subtraction12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (multiplication12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (squaring12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (inversion12() != STS_OK) {
		core_clean();
		return 1;
	}

	if (exponentiation12() != STS_OK) {
		core_clean();
		return 1;
	}
#endif

#ifdef WITH_EB
	r0 = eb_param_set_any_super();
	if (r0 == STS_OK) {
		util_print_banner("Bilinear pairing (genus 1 curve):\n", 0);

		if (pairing1() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

#ifdef WITH_HB
	r1 = hb_param_set_any_super();
	if (r1 == STS_OK) {
		util_print_banner("Bilinear pairing (genus 2 curve):\n", 0);

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

	core_clean();

	return 0;
}
