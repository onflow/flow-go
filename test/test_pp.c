/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * Tests for the quartic extension binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

static int memory2(void) {
	err_t e;
	int code = STS_ERR;
	fp2_t a;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fp2_new(a);
			fp2_free(a);
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
	fp2_t a, b, c;

	fp2_null(a);
	fp2_null(b);
	fp2_null(c);

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);

		TEST_BEGIN("comparison is consistent") {
			fp2_rand(a);
			fp2_rand(b);
			if (fp2_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp2_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_rand(c);
			if (fp2_cmp(a, c) != CMP_EQ) {
				fp2_copy(c, a);
				TEST_ASSERT(fp2_cmp(c, a) == CMP_EQ, end);
			}
			if (fp2_cmp(b, c) != CMP_EQ) {
				fp2_copy(c, b);
				TEST_ASSERT(fp2_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fp2_rand(a);
			fp2_neg(b, a);
			if (fp2_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp2_cmp(b, a) == CMP_NE, end);
			}
			fp2_neg(b, b);
			TEST_ASSERT(fp2_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fp2_rand(a);
			fp2_zero(c);
			TEST_ASSERT(fp2_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fp2_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fp2_rand(a);
			fp2_zero(c);
			TEST_ASSERT(fp2_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fp2_zero(a);
			TEST_ASSERT(fp2_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	return code;
}

static int addition2(void) {
	int code = STS_ERR;
	fp2_t a, b, c, d, e;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);
		fp2_new(d);
		fp2_new(e);

		TEST_BEGIN("addition is commutative") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_add(d, a, b);
			fp2_add(e, b, a);
			TEST_ASSERT(fp2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_rand(c);
			fp2_add(d, a, b);
			fp2_add(d, d, c);
			fp2_add(e, b, c);
			fp2_add(e, a, e);
			TEST_ASSERT(fp2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fp2_rand(a);
			fp2_zero(d);
			fp2_add(e, a, d);
			TEST_ASSERT(fp2_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fp2_rand(a);
			fp2_neg(d, a);
			fp2_add(e, a, d);
			TEST_ASSERT(fp2_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	fp2_free(d);
	fp2_free(e);
	return code;
}

static int subtraction2(void) {
	int code = STS_ERR;
	fp2_t a, b, c, d;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);
		fp2_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_sub(c, a, b);
			fp2_sub(d, b, a);
			fp2_neg(d, d);
			TEST_ASSERT(fp2_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fp2_rand(a);
			fp2_zero(c);
			fp2_sub(d, a, c);
			TEST_ASSERT(fp2_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fp2_rand(a);
			fp2_sub(c, a, a);
			TEST_ASSERT(fp2_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	fp2_free(d);
	return code;
}

static int doubling2(void) {
	int code = STS_ERR;
	fp2_t a, b, c;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);

		TEST_BEGIN("doubling is correct") {
			fp2_rand(a);
			fp2_dbl(b, a);
			fp2_add(c, a, a);
			TEST_ASSERT(fp2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	return code;
}

static int conjugate2(void) {
	int code = STS_ERR;
	fp2_t a, b;

	TRY {
		fp2_new(a);
		fp2_new(b);

		TEST_BEGIN("conjugate is correct") {
			fp2_rand(a);
			fp2_frb(b, a);
			fp2_frb(b, b);
			TEST_ASSERT(fp2_cmp(a, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	return code;
}

static int multiplication2(void) {
	int code = STS_ERR;
	fp2_t a, b, c, d, e, f;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);
		fp2_new(d);
		fp2_new(e);
		fp2_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_mul(d, a, b);
			fp2_mul(e, b, a);
			TEST_ASSERT(fp2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_rand(c);
			fp2_mul(d, a, b);
			fp2_mul(d, d, c);
			fp2_mul(e, b, c);
			fp2_mul(e, a, e);
			TEST_ASSERT(fp2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fp2_rand(a);
			fp2_rand(b);
			fp2_rand(c);
			fp2_add(d, a, b);
			fp2_mul(d, c, d);
			fp2_mul(e, c, a);
			fp2_mul(f, c, b);
			fp2_add(e, e, f);
			TEST_ASSERT(fp2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fp2_rand(a);
			fp2_zero(d);
			fp_set_dig(d[0], 1);
			fp2_mul(e, a, d);
			TEST_ASSERT(fp2_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fp2_rand(a);
			fp2_zero(d);
			fp2_mul(e, a, d);
			TEST_ASSERT(fp2_is_zero(e), end);
		} TEST_END;

		TEST_BEGIN("multiplication by quadratic non-residue is correct") {
			fp2_rand(a);
			fp2_zero(b);
			fp_set_dig(b[1], 1);
			fp2_mul(c, a, b);
			fp2_mul_art(d, a);
			TEST_ASSERT(fp2_cmp(c, d) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	fp2_free(d);
	fp2_free(e);
	fp2_free(f);
	return code;
}

static int squaring2(void) {
	int code = STS_ERR;
	fp2_t a, b, c;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);

		TEST_BEGIN("squaring is correct") {
			fp2_rand(a);
			fp2_mul(b, a, a);
			fp2_sqr(c, a);
			TEST_ASSERT(fp2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	return code;
}

static int inversion2(void) {
	int code = STS_ERR;
	fp2_t a, b, c;

	TRY {
		fp2_new(a);
		fp2_new(b);
		fp2_new(c);

		TEST_BEGIN("inversion is correct") {
			fp2_rand(a);
			fp2_inv(b, a);
			fp2_mul(c, a, b);
			fp2_zero(b);
			fp_set_dig(b[0], 1);
			TEST_ASSERT(fp2_cmp(c, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp2_free(a);
	fp2_free(b);
	fp2_free(c);
	return code;
}

static int memory6(void) {
	err_t e;
	int code = STS_ERR;
	fp6_t a;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fp6_new(a);
			fp6_free(a);
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
	fp6_t a, b, c;

	fp6_null(a);
	fp6_null(b);
	fp6_null(c);

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);

		TEST_BEGIN("comparison is consistent") {
			fp6_rand(a);
			fp6_rand(b);
			if (fp6_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp6_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_rand(c);
			if (fp6_cmp(a, c) != CMP_EQ) {
				fp6_copy(c, a);
				TEST_ASSERT(fp6_cmp(c, a) == CMP_EQ, end);
			}
			if (fp6_cmp(b, c) != CMP_EQ) {
				fp6_copy(c, b);
				TEST_ASSERT(fp6_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fp6_rand(a);
			fp6_neg(b, a);
			if (fp6_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp6_cmp(b, a) == CMP_NE, end);
			}
			fp6_neg(b, b);
			TEST_ASSERT(fp6_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fp6_rand(a);
			fp6_zero(c);
			TEST_ASSERT(fp6_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fp6_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fp6_rand(a);
			fp6_zero(c);
			TEST_ASSERT(fp6_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fp6_zero(a);
			TEST_ASSERT(fp6_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	return code;
}

static int addition6(void) {
	int code = STS_ERR;
	fp6_t a, b, c, d, e;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);
		fp6_new(d);
		fp6_new(e);

		TEST_BEGIN("addition is commutative") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_add(d, a, b);
			fp6_add(e, b, a);
			TEST_ASSERT(fp6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_rand(c);
			fp6_add(d, a, b);
			fp6_add(d, d, c);
			fp6_add(e, b, c);
			fp6_add(e, a, e);
			TEST_ASSERT(fp6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fp6_rand(a);
			fp6_zero(d);
			fp6_add(e, a, d);
			TEST_ASSERT(fp6_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fp6_rand(a);
			fp6_neg(d, a);
			fp6_add(e, a, d);
			TEST_ASSERT(fp6_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	fp6_free(d);
	fp6_free(e);
	return code;
}

static int subtraction6(void) {
	int code = STS_ERR;
	fp6_t a, b, c, d;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);
		fp6_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_sub(c, a, b);
			fp6_sub(d, b, a);
			fp6_neg(d, d);
			TEST_ASSERT(fp6_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fp6_rand(a);
			fp6_zero(c);
			fp6_sub(d, a, c);
			TEST_ASSERT(fp6_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fp6_rand(a);
			fp6_sub(c, a, a);
			TEST_ASSERT(fp6_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	fp6_free(d);
	return code;
}

static int doubling6(void) {
	int code = STS_ERR;
	fp6_t a, b, c;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);

		TEST_BEGIN("doubling is correct") {
			fp6_rand(a);
			fp6_dbl(b, a);
			fp6_add(c, a, a);
			TEST_ASSERT(fp6_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	return code;
}

static int multiplication6(void) {
	int code = STS_ERR;
	fp6_t a, b, c, d, e, f;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);
		fp6_new(d);
		fp6_new(e);
		fp6_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_mul(d, a, b);
			fp6_mul(e, b, a);
			TEST_ASSERT(fp6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_rand(c);
			fp6_mul(d, a, b);
			fp6_mul(d, d, c);
			fp6_mul(e, b, c);
			fp6_mul(e, a, e);
			TEST_ASSERT(fp6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fp6_rand(a);
			fp6_rand(b);
			fp6_rand(c);
			fp6_add(d, a, b);
			fp6_mul(d, c, d);
			fp6_mul(e, c, a);
			fp6_mul(f, c, b);
			fp6_add(e, e, f);
			TEST_ASSERT(fp6_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fp6_zero(d);
			fp_set_dig(d[0][0], 1);
			fp6_mul(e, a, d);
			TEST_ASSERT(fp6_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fp6_zero(d);
			fp6_mul(e, a, d);
			TEST_ASSERT(fp6_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	fp6_free(d);
	fp6_free(e);
	fp6_free(f);
	return code;
}

static int squaring6(void) {
	int code = STS_ERR;
	fp6_t a, b, c;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);

		TEST_BEGIN("squaring is correct") {
			fp6_rand(a);
			fp6_mul(b, a, a);
			fp6_sqr(c, a);
			TEST_ASSERT(fp6_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	return code;
}

static int inversion6(void) {
	int code = STS_ERR;
	fp6_t a, b, c;

	TRY {
		fp6_new(a);
		fp6_new(b);
		fp6_new(c);

		TEST_BEGIN("inversion is correct") {
			fp6_rand(a);
			fp6_inv(b, a);
			fp6_mul(c, a, b);
			fp6_zero(b);
			fp_set_dig(b[0][0], 1);
			TEST_ASSERT(fp6_cmp(c, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp6_free(a);
	fp6_free(b);
	fp6_free(c);
	return code;
}

static int memory12(void) {
	err_t e;
	int code = STS_ERR;
	fp12_t a;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fp12_new(a);
			fp12_free(a);
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
	fp12_t a, b, c;

	fp12_null(a);
	fp12_null(b);
	fp12_null(c);

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);

		TEST_BEGIN("comparison is consistent") {
			fp12_rand(a);
			fp12_rand(b);
			if (fp12_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp12_cmp(b, a) == CMP_NE, end);
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_rand(c);
			if (fp12_cmp(a, c) != CMP_EQ) {
				fp12_copy(c, a);
				TEST_ASSERT(fp12_cmp(c, a) == CMP_EQ, end);
			}
			if (fp12_cmp(b, c) != CMP_EQ) {
				fp12_copy(c, b);
				TEST_ASSERT(fp12_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fp12_rand(a);
			fp12_neg(b, a);
			if (fp12_cmp(a, b) != CMP_EQ) {
				TEST_ASSERT(fp12_cmp(b, a) == CMP_NE, end);
			}
			fp12_neg(b, b);
			TEST_ASSERT(fp12_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fp12_rand(a);
			fp12_zero(c);
			TEST_ASSERT(fp12_cmp(a, c) == CMP_NE, end);
			TEST_ASSERT(fp12_cmp(c, a) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fp12_rand(a);
			fp12_zero(c);
			TEST_ASSERT(fp12_cmp(a, c) == CMP_NE, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fp12_zero(a);
			TEST_ASSERT(fp12_is_zero(a), end);
		}
		TEST_END;

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	return code;
}

static int addition12(void) {
	int code = STS_ERR;
	fp12_t a, b, c, d, e;

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);
		fp12_new(d);
		fp12_new(e);

		TEST_BEGIN("addition is commutative") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_add(d, a, b);
			fp12_add(e, b, a);
			TEST_ASSERT(fp12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_rand(c);
			fp12_add(d, a, b);
			fp12_add(d, d, c);
			fp12_add(e, b, c);
			fp12_add(e, a, e);
			TEST_ASSERT(fp12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fp12_rand(a);
			fp12_zero(d);
			fp12_add(e, a, d);
			TEST_ASSERT(fp12_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fp12_rand(a);
			fp12_neg(d, a);
			fp12_add(e, a, d);
			TEST_ASSERT(fp12_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	fp12_free(d);
	fp12_free(e);
	return code;
}

static int subtraction12(void) {
	int code = STS_ERR;
	fp12_t a, b, c, d;

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);
		fp12_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_sub(c, a, b);
			fp12_sub(d, b, a);
			fp12_neg(d, d);
			TEST_ASSERT(fp12_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fp12_rand(a);
			fp12_zero(c);
			fp12_sub(d, a, c);
			TEST_ASSERT(fp12_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fp12_rand(a);
			fp12_sub(c, a, a);
			TEST_ASSERT(fp12_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	fp12_free(d);
	return code;
}

static int multiplication12(void) {
	int code = STS_ERR;
	fp12_t a, b, c, d, e, f;

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);
		fp12_new(d);
		fp12_new(e);
		fp12_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_mul(d, a, b);
			fp12_mul(e, b, a);
			TEST_ASSERT(fp12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_rand(c);
			fp12_mul(d, a, b);
			fp12_mul(d, d, c);
			fp12_mul(e, b, c);
			fp12_mul(e, a, e);
			TEST_ASSERT(fp12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fp12_rand(a);
			fp12_rand(b);
			fp12_rand(c);
			fp12_add(d, a, b);
			fp12_mul(d, c, d);
			fp12_mul(e, c, a);
			fp12_mul(f, c, b);
			fp12_add(e, e, f);
			TEST_ASSERT(fp12_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fp12_zero(d);
			fp_set_dig(d[0][0][0], 1);
			fp12_mul(e, a, d);
			TEST_ASSERT(fp12_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fp12_zero(d);
			fp12_mul(e, a, d);
			TEST_ASSERT(fp12_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	fp12_free(d);
	fp12_free(e);
	fp12_free(f);
	return code;
}

static int squaring12(void) {
	int code = STS_ERR;
	fp12_t a, b, c;

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);

		TEST_BEGIN("squaring is correct") {
			fp12_rand(a);
			fp12_mul(b, a, a);
			fp12_sqr(c, a);
			TEST_ASSERT(fp12_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	return code;
}

static int inversion12(void) {
	int code = STS_ERR;
	fp12_t a, b, c;

	TRY {
		fp12_new(a);
		fp12_new(b);
		fp12_new(c);

		TEST_BEGIN("inversion is correct") {
			fp12_rand(a);
			fp12_inv(b, a);
			fp12_mul(c, a, b);
			fp12_zero(b);
			fp_set_dig(b[0][0][0], 1);
			TEST_ASSERT(fp12_cmp(c, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp12_free(a);
	fp12_free(b);
	fp12_free(c);
	return code;
}

static int memory(void) {
	err_t e;
	int code = STS_ERR;
	ep2_t a;

	ep2_null(a);

	TRY {
		TEST_BEGIN("memory can be allocated") {
			ep2_new(a);
			ep2_free(a);
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

int util(void) {
	int code = STS_ERR;
	ep2_t a, b, c;

	ep2_null(a);
	ep2_null(b);
	ep2_null(c);

	TRY {
		ep2_new(a);
		ep2_new(b);
		ep2_new(c);

		TEST_BEGIN("comparison is consistent") {
			ep2_rand(a);
			ep2_rand(b);
			TEST_ASSERT(ep2_cmp(a, b) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			ep2_rand(a);
			ep2_rand(b);
			if (ep2_cmp(a, c) != CMP_EQ) {
				ep2_copy(c, a);
				TEST_ASSERT(ep2_cmp(c, a) == CMP_EQ, end);
			}
			if (ep2_cmp(b, c) != CMP_EQ) {
				ep2_copy(c, b);
				TEST_ASSERT(ep2_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation and comparison are consistent") {
			//ep2_rand(a);
			//ep2_neg(b, a);
			//TEST_ASSERT(ep2_cmp(a, b) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN
				("assignment to random/infinity and comparison are consistent")
		{
			ep2_rand(a);
			ep2_set_infty(c);
			TEST_ASSERT(ep2_cmp(a, c) != CMP_EQ, end);
			TEST_ASSERT(ep2_cmp(c, a) != CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to infinity and infinity test are consistent") {
			ep2_set_infty(a);
			TEST_ASSERT(ep2_is_infty(a), end);
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
	ep2_t a, b, c, d, e;

	ep2_null(a);
	ep2_null(b);
	ep2_null(c);
	ep2_null(d);
	ep2_null(e);

	TRY {
		ep2_new(a);
		ep2_new(b);
		ep2_new(c);
		ep2_new(d);
		ep2_new(e);

		TEST_BEGIN("point addition is commutative") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_add(d, a, b);
			ep2_add(e, b, a);
			ep2_norm(d, d);
			ep2_norm(e, e);
			TEST_ASSERT(ep2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition is associative") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_rand(c);
			ep2_add(d, a, b);
			ep2_add(d, d, c);
			ep2_add(e, b, c);
			ep2_add(e, e, a);
			ep2_norm(d, d);
			ep2_norm(e, e);
			TEST_ASSERT(ep2_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition has identity") {
			ep2_rand(a);
			ep2_set_infty(d);
			ep2_add(e, a, d);
			TEST_ASSERT(ep2_cmp(e, a) == CMP_EQ, end);
			ep2_add(e, d, a);
			TEST_ASSERT(ep2_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition has inverse") {
			//          ep2_rand(a);
			//          ep2_neg(d, a);
			//          ep2_add(e, a, d);
			//          TEST_ASSERT(ep2_is_infty(e), end);
		} TEST_END;

#if EP_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("point addition in affine coordinates is correct") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_add(d, a, b);
			ep2_norm(d, d);
			ep2_add_basic(e, a, b);
			TEST_ASSERT(ep2_cmp(e, d) == CMP_EQ, end);
		} TEST_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
		TEST_BEGIN("point addition in projective coordinates is correct") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_add_projc(a, a, b);
			ep2_rand(b);
			ep2_rand(c);
			ep2_add_projc(b, b, c);
			/* a and b in projective coordinates. */
			ep2_add_projc(d, a, b);
			ep2_norm(d, d);
			ep2_norm(a, a);
			ep2_norm(b, b);
			ep2_add(e, a, b);
			ep2_norm(e, e);
			TEST_ASSERT(ep2_cmp(e, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition in mixed coordinates (z2 = 1) is correct") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_add_projc(a, a, b);
			ep2_rand(b);
			/* a and b in projective coordinates. */
			ep2_add_projc(d, a, b);
			ep2_norm(d, d);
			/* a in affine coordinates. */
			ep2_norm(a, a);
			ep2_add(e, a, b);
			ep2_norm(e, e);
			TEST_ASSERT(ep2_cmp(e, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point addition in mixed coordinates (z1,z2 = 1) is correct") {
			ep2_rand(a);
			ep2_rand(b);
			ep2_norm(a, a);
			ep2_norm(b, b);
			/* a and b in affine coordinates. */
			ep2_add(d, a, b);
			ep2_norm(d, d);
			ep2_add_projc(e, a, b);
			ep2_norm(e, e);
			TEST_ASSERT(ep2_cmp(e, d) == CMP_EQ, end);
		} TEST_END;
#endif

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep2_free(a);
	ep2_free(b);
	ep2_free(c);
	ep2_free(d);
	ep2_free(e);
	return code;
}

int doubling(void) {
	int code = STS_ERR;
	ep2_t a, b, c;

	ep2_null(a);
	ep2_null(b);
	ep2_null(c);

	TRY {
		ep2_new(a);
		ep2_new(b);
		ep2_new(c);

		TEST_BEGIN("point doubling is correct") {
			ep2_rand(a);
			ep2_add(b, a, a);
			ep2_norm(b, b);
			ep2_dbl(c, a);
			ep2_norm(c, c);
			TEST_ASSERT(ep2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if EP_ADD == BASIC || !defined(STRIP)
		TEST_BEGIN("point doubling in affine coordinates is correct") {
			ep2_rand(a);
			ep2_dbl(b, a);
			ep2_norm(b, b);
			ep2_dbl_basic(c, a);
			TEST_ASSERT(ep2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if EP_ADD == PROJC || !defined(STRIP)
		TEST_BEGIN("point doubling in projective coordinates is correct") {
			ep2_rand(a);
			ep2_dbl_projc(a, a);
			/* a in projective coordinates. */
			ep2_dbl_projc(b, a);
			ep2_norm(b, b);
			ep2_norm(a, a);
			ep2_dbl(c, a);
			ep2_norm(c, c);
			TEST_ASSERT(ep2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("point doubling in mixed coordinates (z1 = 1) is correct") {
			ep2_rand(a);
			ep2_dbl_projc(b, a);
			ep2_norm(b, b);
			ep2_dbl(c, a);
			ep2_norm(c, c);
			TEST_ASSERT(ep2_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	ep2_free(a);
	ep2_free(b);
	ep2_free(c);
	return code;
}

int main(void) {
	core_init();
	ep2_t p;
	ep_t q, t;
	bn_t x;
	fp12_t r1, r2, f;

	ep2_new(p);
	ep_new(q);
	ep_new(t);
	bn_new(x);
	fp12_new(r1);
	fp12_new(r2);
	fp12_new(f);

	if (fp_param_set_any_tower() != STS_OK) {
		THROW(ERR_NO_FIELD);
		core_clean();
		return 1;
	}

	util_print_banner("Tests for the PP module", 0);

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

	if (doubling2() != STS_OK) {
		core_clean();
		return 1;
	}

	if (conjugate2() != STS_OK) {
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

	util_print_banner("Sextic extension:", 0);
	util_print_banner("Utilities:", 1);

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

	if (doubling6() != STS_OK) {
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

	util_print_banner("Dodecic extension:", 0);
	util_print_banner("Utilities:", 1);

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

	if (ep_param_set_any_pairf() == STS_ERR) {
		THROW(ERR_NO_CURVE);
		core_clean();
		return 1;
	}
	ep2_curve_set_twist(1);

	util_print_banner("Quadratic twist:", 0);
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

	if (addition() != STS_OK) {
		core_clean();
		return 1;
	}

	if (doubling() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Bilinear pairing:\n", 0);

	char *xp0 =
			"1822AA754FAFAFF95FE37842D7D5DECE88305EC19B363F6681DF06BF405F02B4";
	char *xp1 =
			"1AB4CC8A133A7AA970AADAE37C20D1C7191279CBA02830AFC64C19B50E8B1997";
	char *yp0 =
			"16737CF6F9DEC5895A7E5A6D60316763FB6638A0A82F26888E909DA86F7F84BA";
	char *yp1 =
			"5B6DB6FF5132FB917E505627E7CCC12E0CE9FCC4A59805B3B730EE0EC44E43C";
	char *sx = "-4080000000000001";
	char *f0 =
			"1B377619212E7C8CB6499B50A846953F850974924D3F77C2E17DE6C06F2A6DE9";
	char *f1 =
			"9EBEE691ED1837503EAB22F57B96AC8DC178B6DB2C08850C582193F90D5922A";

	ep_copy(q, ep_curve_get_gen());
	fp_read(p->x[0], xp0, strlen(xp0), 16);
	fp_read(p->x[1], xp1, strlen(xp1), 16);
	fp_read(p->y[0], yp0, strlen(yp0), 16);
	fp_read(p->y[1], yp1, strlen(yp1), 16);
	fp2_zero(p->z);
	fp_set_dig(p->z[0], 1);
	fp12_zero(f);
	fp_read(f[1][0][0], f0, strlen(f0), 16);
	fp_read(f[1][0][1], f1, strlen(f1), 16);
	bn_read_str(x, sx, strlen(sx), 16);

	TEST_BEGIN("rate pairing is bilinear") {
		ep_dbl(t, q);
		ep_norm(t, t);
		pp_pair_rate(r1, p, t, x, f);
		ep2_dbl(p, p);
		ep2_norm(p, p);
		pp_pair_rate(r2, p, q, x, f);
		TEST_ASSERT(fp12_cmp(r1, r2) == CMP_EQ, end);
	} TEST_END;

	ep2_free(p);
	ep_free(q);
	ep_free(t);
	bn_free(x);
	fp12_free(r1);
	fp12_free(r2);
	fp12_free(f);
  end:
	core_clean();

	return 0;
}
