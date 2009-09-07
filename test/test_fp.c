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
 * Tests for the prime field arithmetic module.
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
	fp_t a = NULL;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			fp_new(a);
			fp_free(a);
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
	int bits, code = STS_ERR;
	char str[1000];
	fp_t a = NULL, b = NULL, c = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);

		TEST_BEGIN("comparison is consistent") {
			fp_rand(a);
			fp_rand(b);
			if (fp_cmp(a, b) != CMP_EQ) {
				if (fp_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(fp_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(fp_cmp(b, a) == CMP_GT, end);
				}
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			fp_rand(a);
			fp_rand(c);
			fp_rand(b);
			if (fp_cmp(a, c) != CMP_EQ) {
				fp_copy(c, a);
				TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
			}
			if (fp_cmp(b, c) != CMP_EQ) {
				fp_copy(c, b);
				TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("negation is consistent") {
			fp_rand(a);
			fp_neg(b, a);
			if (fp_cmp(a, b) != CMP_EQ) {
				if (fp_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(fp_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(fp_cmp(b, a) == CMP_GT, end);
				}
			}
			fp_neg(b, b);
			TEST_ASSERT(fp_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			fp_rand(a);
			fp_zero(c);
			TEST_ASSERT(fp_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(fp_cmp(c, a) == CMP_LT, end);
		}
		TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			fp_rand(a);
			fp_rand(b);
			fp_zero(c);
			TEST_ASSERT(fp_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(fp_cmp(b, c) == CMP_GT, end);
			if (fp_cmp(a, b) != CMP_EQ) {
				if (fp_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(fp_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(fp_cmp(b, a) == CMP_GT, end);
				}
			}
		}
		TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			fp_zero(a);
			TEST_ASSERT(fp_is_zero(a), end);
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("bit setting and testing are consistent") {
			fp_zero(a);
			fp_set_bit(a, bits, 1);
			TEST_ASSERT(fp_test_bit(a, bits), end);
			bits = (bits + 1) % FP_BITS;
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("bit assignment and counting are consistent") {
			fp_zero(a);
			fp_set_bit(a, bits, 1);
			TEST_ASSERT(fp_bits(a) == bits + 1, end);
			bits = (bits + 1) % FP_BITS;
		}
		TEST_END;

		TEST_BEGIN("reading and writing a prime field element are consistent") {
			fp_rand(a);
			fp_write(str, sizeof(str), a, 16);
			fp_read(b, str, sizeof(str), 16);
			TEST_ASSERT(fp_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		/*TEST_BEGIN("getting the size of a prime field element is correct") {
			fp_rand(a);
			fp_size(&bits, a, 2);
			bits--;
			printf("%d %d\n", bits, fp_bits(a));
			fp_print(a);
			TEST_ASSERT(bits == fp_bits(a), end);
		}
		TEST_END;*/
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	return code;
}

static int addition(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);
		fp_new(d);
		fp_new(e);

		TEST_BEGIN("addition is commutative") {
			fp_rand(a);
			fp_rand(b);
			fp_add(d, a, b);
			fp_add(e, b, a);
			TEST_ASSERT(fp_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			fp_rand(a);
			fp_rand(b);
			fp_rand(c);
			fp_add(d, a, b);
			fp_add(d, d, c);
			fp_add(e, b, c);
			fp_add(e, a, e);
			TEST_ASSERT(fp_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			fp_rand(a);
			fp_zero(d);
			fp_add(e, a, d);
			TEST_ASSERT(fp_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			fp_rand(a);
			fp_neg(d, a);
			fp_add(e, a, d);
			TEST_ASSERT(fp_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	fp_free(d);
	fp_free(e);
	return code;
}

static int subtraction(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL, d = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);
		fp_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			fp_rand(a);
			fp_rand(b);
			fp_sub(c, a, b);
			fp_sub(d, b, a);
			fp_neg(d, d);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			fp_rand(a);
			fp_zero(c);
			fp_sub(d, a, c);
			TEST_ASSERT(fp_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			fp_rand(a);
			fp_sub(c, a, a);
			TEST_ASSERT(fp_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	fp_free(d);
	return code;
}

static int multiplication(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL, f = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);
		fp_new(d);
		fp_new(e);
		fp_new(f);

		TEST_BEGIN("multiplication is commutative") {
			fp_rand(a);
			fp_rand(b);
			fp_mul(d, a, b);
			fp_mul(e, b, a);
			TEST_ASSERT(fp_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			fp_rand(a);
			fp_rand(b);
			fp_rand(c);
			fp_mul(d, a, b);
			fp_mul(d, d, c);
			fp_mul(e, b, c);
			fp_mul(e, a, e);
			TEST_ASSERT(fp_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			fp_rand(a);
			fp_rand(b);
			fp_rand(c);
			fp_add(d, a, b);
			fp_mul(d, c, d);
			fp_mul(e, c, a);
			fp_mul(f, c, b);
			fp_add(e, e, f);
			TEST_ASSERT(fp_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			fp_rand(a);
			fp_set_dig(d, 1);
			fp_mul(e, a, d);
			TEST_ASSERT(fp_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			fp_rand(a);
			fp_zero(d);
			fp_mul(e, a, d);
			TEST_ASSERT(fp_is_zero(e), end);
		} TEST_END;

#if FP_MUL == BASIC || !defined(STRIP)
		TEST_BEGIN("basic multiplication is correct") {
			fp_rand(a);
			fp_rand(b);
			fp_mul(c, a, b);
			fp_mul_basic(d, a, b);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if FP_MUL == COMBA || !defined(STRIP)
		TEST_BEGIN("comba multiplication is correct") {
			fp_rand(a);
			fp_rand(b);
			fp_mul(c, a, b);
			fp_mul_comba(d, a, b);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if FP_KARAT > 0 || !defined(STRIP)
		TEST_BEGIN("karatsuba multiplication is correct") {
			fp_rand(a);
			fp_rand(b);
			fp_mul(c, a, b);
			fp_mul_karat(d, a, b);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	fp_free(d);
	fp_free(e);
	fp_free(f);
	return code;
}

static int squaring(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);

		TEST_BEGIN("squaring is correct") {
			fp_rand(a);
			fp_mul(b, a, a);
			fp_sqr(c, a);
			TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if FP_SQR == BASIC || !defined(STRIP)
		TEST_BEGIN("basic squaring is correct") {
			fp_rand(a);
			fp_sqr(b, a);
			fp_sqr_basic(c, a);
			TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FP_SQR == COMBA || !defined(STRIP)
		TEST_BEGIN("comba squaring is correct") {
			fp_rand(a);
			fp_sqr(b, a);
			fp_sqr_comba(c, a);
			TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if FP_KARAT > 0 || !defined(STRIP)
		TEST_BEGIN("karatsuba squaring is correct") {
			fp_rand(a);
			fp_sqr(b, a);
			fp_sqr_karat(c, a);
			TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	return code;
}

static int doubling_halving(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);

		TEST_BEGIN("doubling is consistent") {
			fp_rand(a);
			fp_add(b, a, a);
			fp_dbl(c, a);
			TEST_ASSERT(fp_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("halving is consistent") {
			fp_rand(a);
			fp_hlv(b, a);
			fp_dbl(c, b);
			if (!fp_is_even(a)) {
				fp_add_dig(c, c, 1);
			}
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		util_print("FATAL ERROR!\n");
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	return code;
}

static int shifting(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL;
	dv_t d = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);
		dv_new(d);

		TEST_BEGIN("shifting by 1 bit is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			fp_lsh(b, a, 1);
			fp_rsh(c, b, 1);
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 bits is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			fp_lsh(b, a, 2);
			fp_rsh(c, b, 2);
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by half digit is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			fp_lsh(b, a, FP_DIGIT / 2);
			fp_rsh(c, b, FP_DIGIT / 2);
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 1 digit is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			fp_lsh(b, a, FP_DIGIT);
			fp_rsh(c, b, FP_DIGIT);
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 digits is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			a[FP_DIGS - 2] = 0;
			fp_lsh(b, a, 2 * FP_DIGIT);
			fp_rsh(c, b, 2 * FP_DIGIT);
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 1 digit and half is consistent") {
			fp_rand(a);
			a[FP_DIGS - 1] = 0;
			a[FP_DIGS - 2] = 0;
			fp_lsh(b, a, FP_DIGIT + FP_DIGIT / 2);
			fp_rsh(c, b, (FP_DIGIT + FP_DIGIT / 2));
			TEST_ASSERT(fp_cmp(c, a) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	dv_free(d);
	return code;
}

static int digit(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL, d = NULL;
	dig_t g;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);
		fp_new(d);

		TEST_BEGIN("addition of a single digit is consistent") {
			fp_rand(a);
			fp_rand(b);
			for (int j = 1; j < FP_DIGS; j++)
				b[j] = 0;
			g = b[0];
			fp_add(c, a, b);
			fp_add_dig(d, a, g);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("subtraction of a single digit is consistent") {
			fp_rand(a);
			fp_rand(b);
			for (int j = 1; j < FP_DIGS; j++)
				b[j] = 0;
			g = b[0];
			fp_sub(c, a, b);
			fp_sub_dig(d, a, g);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication by a single digit is consistent") {
			fp_rand(a);
			fp_rand(b);
			for (int j = 1; j < FP_DIGS; j++)
				b[j] = 0;
			g = b[0];
			fp_prime_conv_dig(b, g);
			fp_mul(c, a, b);
			fp_mul_dig(d, a, g);
			TEST_ASSERT(fp_cmp(c, d) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	fp_free(d);
	return code;
}

static int inversion(void) {
	int code = STS_ERR;
	fp_t a = NULL, b = NULL, c = NULL;

	TRY {
		fp_new(a);
		fp_new(b);
		fp_new(c);

		TEST_BEGIN("inversion is correct") {
			fp_rand(a);
			fp_inv(b, a);
			fp_mul(c, a, b);
			fp_prime_conv_dig(b, 1);
			TEST_ASSERT(fp_cmp(c, b) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	fp_free(a);
	fp_free(b);
	fp_free(c);
	return code;
}

int main(void) {
	core_init();
	bn_t modulus;

	bn_new(modulus);
	bn_gen_prime(modulus, FP_BITS);
	fp_prime_set(modulus);
	bn_free(modulus);

	util_print("Prime field order: ");
	fp_print(fp_prime_get());
	util_print("\n");

	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util() != STS_OK) {
		core_clean();
		return 1;
	}

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

	if (squaring() != STS_OK) {
		core_clean();
		return 1;
	}

	if (doubling_halving() != STS_OK) {
		core_clean();
		return 1;
	}

	if (shifting() != STS_OK) {
		core_clean();
		return 1;
	}

	if (inversion() != STS_OK) {
		core_clean();
		return 1;
	}

	 /* if (reduction() != STS_OK) {
	 * core_clean();
	 * return 1;
	 * } */

	if (digit() != STS_OK) {
		core_clean();
		return 1;
	}

	core_clean();
	return 0;
}
