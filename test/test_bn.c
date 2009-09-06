/*
 * Copyright 2007-2009 RELIC Project
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
 * Tests for the multiple precision integer arithmetic module.
 *
 * @version $Id: test_bn.c 36 2009-06-03 18:15:10Z dfaranha $
 * @ingroup test
 */

#include <stdlib.h>
#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

void bn_new_impl(bn_t *a) {
	bn_new(*a);
}

int memory(void) {
	err_t e;
	int code = STS_ERR;
	bn_t a = NULL;

	TRY {
		TEST_BEGIN("memory can be allocated") {
			bn_new_impl(&a);
			bn_free(a);
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
	int bits;
	char str[BN_DIGS * sizeof(dig_t) * 3 + 1];
	dig_t digit;
	unsigned char bin[BN_DIGS * sizeof(dig_t)];
	dig_t raw[BN_DIGS];
	bn_t a = NULL, b = NULL, c = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		TEST_BEGIN("comparison is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			if (bn_cmp(a, b) != CMP_EQ) {
				if (bn_cmp(a, b) == CMP_GT) {
					TEST_ASSERT(bn_cmp(b, a) == CMP_LT, end);
				} else {
					TEST_ASSERT(bn_cmp(b, a) == CMP_GT, end);
				}
			}
		}
		TEST_END;

		TEST_BEGIN("copy and comparison are consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_rand(c, BN_POS, BN_BITS);
			if (bn_cmp(a, c) != CMP_EQ) {
				bn_copy(c, a);
				TEST_ASSERT(bn_cmp(c, a) == CMP_EQ, end);
			}
			if (bn_cmp(b, c) != CMP_EQ) {
				bn_copy(c, b);
				TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
			}
		}
		TEST_END;

		TEST_BEGIN("absolute, negation and comparison are consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_neg(b, a);
			bn_abs(a, b);
			TEST_ASSERT(bn_cmp(a, b) == CMP_GT, end);
			TEST_ASSERT(bn_cmp(b, a) == CMP_LT, end);
			TEST_ASSERT(bn_cmp_abs(a, b) == CMP_EQ, end);
			TEST_ASSERT(bn_cmp_dig(a, (dig_t)0) == CMP_GT, end);
			TEST_ASSERT(bn_cmp_dig(b, (dig_t)0) == CMP_LT, end);
		} TEST_END;

		TEST_BEGIN("signal test is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_NEG, BN_BITS);
			TEST_ASSERT(bn_sign(a) == BN_POS, end);
			TEST_ASSERT(bn_sign(b) == BN_NEG, end);
		} TEST_END;

		TEST_BEGIN("assignment to zero and comparison are consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_NEG, BN_BITS);
			bn_zero(c);
			TEST_ASSERT(bn_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(bn_cmp(c, a) == CMP_LT, end);
			TEST_ASSERT(bn_cmp(b, c) == CMP_LT, end);
			TEST_ASSERT(bn_cmp(c, b) == CMP_GT, end);
			TEST_ASSERT(bn_cmp_dig(a, (dig_t)0) == CMP_GT, end);
			TEST_ASSERT(bn_cmp_dig(b, (dig_t)0) == CMP_LT, end);
			TEST_ASSERT(bn_cmp_dig(c, (dig_t)0) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("assignment to zero and zero test are consistent") {
			bn_zero(c);
			TEST_ASSERT(bn_is_zero(c), end);
			TEST_ASSERT(bn_cmp_dig(c, (dig_t)0) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("oddness test is correct") {
			bn_set_dig(a, 2);
			bn_set_dig(b, 1);
			TEST_ASSERT(bn_is_even(a) == 1, end);
			TEST_ASSERT(bn_is_even(b) == 0, end);
		} TEST_END;

		bits = 0;
		TEST_BEGIN("assignment and bit counting are consistent") {
			bn_set_2b(a, bits);
			TEST_ASSERT(bits + 1 == bn_bits(a), end);
			bits = (bits + 1) % BN_BITS;
		} TEST_END;

		bits = 0;
		TEST_BEGIN("bit setting and testing are consistent") {
			bn_zero(a);
			bn_set_bit(a, bits, 1);
			TEST_ASSERT(bn_test_bit(a, bits), end);
			bits = (bits + 1) % BN_BITS;
		}
		TEST_END;

		bits = 0;
		TEST_BEGIN("bit setting and getting are consistent") {
			bn_zero(a);
			bn_set_bit(a, bits, 1);
			TEST_ASSERT(bn_get_bit(a, bits) == 1, end);
			bn_set_bit(a, bits, 0);
			TEST_ASSERT(bn_get_bit(a, bits) == 0, end);
			bits = (bits + 1) % BN_BITS;
		}
		TEST_END;

		TEST_BEGIN("reading and writing the first digit are consistent") {
			bn_rand(a, BN_POS, BN_DIGIT);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&digit, a);
			bn_set_dig(b, digit);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("assignment to a constant and comparison are consistent") {
			bn_set_dig(a, 2);
			bn_set_dig(b, 1);
			TEST_ASSERT(bn_cmp(a, b) == CMP_GT, end);
			TEST_ASSERT(bn_cmp(b, a) == CMP_LT, end);
			TEST_ASSERT(bn_cmp_dig(a, (dig_t)0) == CMP_GT, end);
			TEST_ASSERT(bn_cmp_dig(b, (dig_t)0) == CMP_GT, end);
		} TEST_END;

		TEST_BEGIN("assignment to random and comparison are consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_NEG, BN_BITS);
			bn_zero(c);
			TEST_ASSERT(bn_cmp(a, c) == CMP_GT, end);
			TEST_ASSERT(bn_cmp(b, c) == CMP_LT, end);
			TEST_ASSERT(bn_cmp_dig(a, (dig_t)0) == CMP_GT, end);
			TEST_ASSERT(bn_cmp_dig(b, (dig_t)0) == CMP_LT, end);
		} TEST_END;

		bits = 0;
		TEST_BEGIN("different forms of assignment are consistent") {
			bn_set_dig(a, (dig_t)(1) << (dig_t)bits);
			bn_set_2b(b, bits);
			bits++;
			bits %= (BN_DIGIT);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("reading and writing a positive number are consistent") {
			int len = BN_DIGS * sizeof(dig_t);
			int sign;
			bn_rand(a, BN_POS, BN_BITS);
			bn_write_str(str, sizeof(str), a, 10);
			bn_read_str(b, str, sizeof(str), 10);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
			bn_write_bin(bin, &len, &sign, a);
			bn_read_bin(b, bin, len, sign);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
			len = BN_DIGS;
			bn_write_raw(raw, &len, &sign, a);
			bn_read_raw(b, raw, len, sign);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("getting the size of a positive number is correct") {
			int len;
			bn_rand(a, BN_POS, BN_BITS);
			bn_size_str(&len, a, 2);
			len--;
			TEST_ASSERT(len == bn_bits(a), end);
			bn_size_bin(&len, a);
			bits = (bn_bits(a) % 8 == 0 ? bn_bits(a) / 8 : bn_bits(a) / 8 + 1);
			TEST_ASSERT(bits == len, end);
			bn_size_raw(&len, a);
			bits = a->used;
			TEST_ASSERT(bits == len, end);
		}
		TEST_END;

		TEST_BEGIN("reading and writing a negative number are consistent") {
			int len = BN_DIGS * sizeof(dig_t);
			int sign;
			bn_rand(a, BN_NEG, BN_BITS);
			bn_write_str(str, sizeof(str), a, 10);
			bn_read_str(b, str, sizeof(str), 10);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
			bn_write_bin(bin, &len, &sign, a);
			bn_read_bin(b, bin, len, sign);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
			len = BN_DIGS;
			bn_write_raw(raw, &len, &sign, a);
			bn_read_raw(b, raw, len, sign);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("getting the size of a negative number is correct") {
			int len;
			bn_rand(a, BN_NEG, BN_BITS);
			bn_size_str(&len, a, 2);
			len -= 2;
			TEST_ASSERT(len == bn_bits(a), end);
			bn_size_bin(&len, a);
			bits = (bn_bits(a) % 8 == 0 ? bn_bits(a) / 8 : bn_bits(a) / 8 + 1);
			TEST_ASSERT(bits == len, end);
			bn_size_raw(&len, a);
			bits = a->used;
			TEST_ASSERT(bits == len, end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	return code;
}

int addition(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);

		TEST_BEGIN("addition is commutative") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_add(d, a, b);
			bn_add(e, b, a);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition is associative") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_rand(c, BN_POS, BN_BITS);
			bn_add(d, a, b);
			bn_add(d, d, c);
			bn_add(e, b, c);
			bn_add(e, a, e);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has identity") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_zero(d);
			bn_add(e, a, d);
			TEST_ASSERT(bn_cmp(e, a) == CMP_EQ, end);
			bn_add(e, d, a);
			TEST_ASSERT(bn_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("addition has inverse") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_neg(d, a);
			bn_add(e, a, d);
			TEST_ASSERT(bn_is_zero(e), end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	return code;
}

int subtraction(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);

		TEST_BEGIN("subtraction is anti-commutative") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_sub(c, a, b);
			bn_sub(d, b, a);
			TEST_ASSERT(bn_cmp_abs(c, d) == CMP_EQ, end);
			if (!bn_is_zero(c))
				TEST_ASSERT(bn_sign(c) != bn_sign(d), end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has identity") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_zero(c);
			bn_sub(d, a, c);
			TEST_ASSERT(bn_cmp(d, a) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("subtraction has inverse") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_sub(c, a, a);
			TEST_ASSERT(bn_is_zero(c), end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	return code;
}

int multiplication(void) {
	int code = STS_ERR;

	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL, f = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);
		bn_new(f);

		TEST_BEGIN("multiplication is commutative") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_mul(d, a, b);
			bn_mul(e, b, a);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is associative") {
			bn_rand(a, BN_POS, BN_BITS / 3);
			bn_rand(b, BN_POS, BN_BITS / 3);
			bn_rand(c, BN_POS, BN_BITS / 3);
			bn_mul(d, a, b);
			bn_mul(d, d, c);
			bn_mul(e, b, c);
			bn_mul(e, a, e);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication is distributive") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_rand(c, BN_POS, BN_BITS / 2);
			bn_add(d, a, b);
			bn_mul(d, c, d);
			bn_mul(e, c, a);
			bn_mul(f, c, b);
			bn_add(e, e, f);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has identity") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_set_dig(d, (dig_t)1);
			bn_mul(e, a, d);
			TEST_ASSERT(bn_cmp(e, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication has zero property") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_zero(d);
			bn_mul(e, a, d);
			TEST_ASSERT(bn_is_zero(e), end);
		} TEST_END;

		TEST_BEGIN("multiplication has negation property") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_set_dig(d, 1);
			bn_neg(d, d);
			bn_mul(e, a, d);
			TEST_ASSERT(bn_cmp_abs(e, a) == CMP_EQ, end);
			TEST_ASSERT(bn_sign(e) == BN_NEG, end);
		} TEST_END;

		TEST_BEGIN("multiplication by a positive number preserves order") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_rand(c, BN_POS, BN_BITS / 2);
			int t = bn_cmp(a, b);
			bn_mul(d, c, a);
			bn_mul(e, c, b);
			TEST_ASSERT(bn_cmp(d, e) == t, end);
		} TEST_END;

		TEST_BEGIN("multiplication by a negative number reverses order") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_rand(c, BN_POS, BN_BITS / 2);
			int t = bn_cmp(a, b);
			bn_neg(d, c);
			bn_mul(e, d, a);
			bn_mul(d, d, b);
			if (t != CMP_EQ) {
				TEST_ASSERT(bn_cmp(e, d) != t, end);
			}
		}
		TEST_END;

#if BN_MUL == BASIC || !defined(STRIP)
		TEST_BEGIN("basic multiplication is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_mul(c, a, b);
			bn_mul_basic(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if BN_MUL == COMBA || !defined(STRIP)
		TEST_BEGIN("comba multiplication is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_mul(c, a, b);
			bn_mul_comba(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if BN_KARAT > 0 || !defined(STRIP)
		TEST_BEGIN("karatsuba multiplication is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_mul(c, a, b);
			bn_mul_karat(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	bn_free(f);
	return code;
}

int squaring(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		TEST_BEGIN("squaring is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_mul(b, a, a);
			bn_sqr(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

#if BN_SQR == BASIC || !defined(STRIP)
		TEST_BEGIN("basic squaring is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_sqr(b, a);
			bn_sqr_basic(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if BN_SQRL == COMBA || !defined(STRIP)
		TEST_BEGIN("comba squaring is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_sqr(b, a);
			bn_sqr_comba(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

#if BN_KARAT > 0 || !defined(STRIP)
		TEST_BEGIN("karatsuba squaring is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_sqr(b, a);
			bn_sqr_karat(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
#endif

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	return code;
}

int doubling_halving(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		TEST_BEGIN("doubling is consistent") {
			bn_rand(a, BN_POS, BN_BITS - 1);
			bn_add(b, a, a);
			bn_dbl(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("halving is consistent") {
			bn_rand(a, BN_POS, BN_BITS - 1);
			bn_dbl(b, a);
			bn_hlv(c, b);
			TEST_ASSERT(bn_cmp(c, a) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	return code;
}

int shifting(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		TEST_BEGIN("shifting by 1 bit is consistent") {
			bn_rand(a, BN_POS, BN_BITS - 1);
			bn_lsh(b, a, 1);
			bn_dbl(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
			bn_rsh(b, a, 1);
			bn_hlv(c, a);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
			bn_lsh(b, a, 1);
			bn_rsh(c, b, 1);
			TEST_ASSERT(bn_cmp(a, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 bits is consistent") {
			bn_rand(a, BN_POS, BN_BITS - 2);
			bn_lsh(b, a, 2);
			bn_rsh(c, b, 2);
			TEST_ASSERT(bn_cmp(a, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by half digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT / 2);
			bn_lsh(b, a, BN_DIGIT / 2);
			bn_rsh(c, b, BN_DIGIT / 2);
			TEST_ASSERT(bn_cmp(a, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 1 digit is consistent") {
			bn_rand(a, BN_POS, BN_DIGIT);
			bn_lsh(b, a, BN_DIGIT);
			bn_rsh(c, b, BN_DIGIT);
			TEST_ASSERT(bn_cmp(a, c) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("shifting by 2 digits is consistent") {
			bn_rand(a, BN_POS, BN_BITS - 2 * BN_DIGIT);
			bn_lsh(b, a, 2 * BN_DIGIT);
			bn_rsh(c, b, 2 * BN_DIGIT);
			TEST_ASSERT(bn_cmp(a, c) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("shifting by 1 digit and half is consistent") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT - BN_DIGIT / 2);
			bn_lsh(b, a, BN_DIGIT + BN_DIGIT / 2);
			bn_copy(c, a);
			for (int j = 0; j < (int)(BN_DIGIT + BN_DIGIT / 2); j++)
				bn_dbl(c, c);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
			bn_rsh(b, a, (BN_DIGIT + BN_DIGIT / 2));
			bn_copy(c, a);
			for (int j = 0; j < (int)(BN_DIGIT + BN_DIGIT / 2); j++)
				bn_hlv(c, c);
			TEST_ASSERT(bn_cmp(b, c) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	return code;
}

int division(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);

		TEST_BEGIN("trivial division is correct") {
			bn_rand(a, BN_POS, BN_BITS / 2);
			bn_rand(b, BN_POS, BN_BITS);
			bn_div_norem(e, a, b);
			bn_div_basic(c, d, a, b);
			TEST_ASSERT(bn_cmp(e, c) == CMP_EQ, end);
			bn_mul(e, c, b);
			bn_add(e, e, d);
			TEST_ASSERT(bn_cmp(a, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("division is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_div_norem(e, a, b);
			bn_div_basic(c, d, a, b);
			TEST_ASSERT(bn_cmp(e, c) == CMP_EQ, end);
			bn_mul(e, c, b);
			bn_add(e, e, d);
			TEST_ASSERT(bn_cmp(a, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("negative division is correct") {
			bn_rand(a, BN_NEG, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_div_norem(e, a, b);
			bn_div_basic(c, d, a, b);
			TEST_ASSERT(bn_cmp(e, c) == CMP_EQ, end);
			bn_mul(e, c, b);
			bn_add(e, e, d);
			TEST_ASSERT(bn_cmp(a, e) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("division by negative is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_NEG, BN_BITS / 2);
			bn_div_norem(e, a, b);
			bn_div_basic(c, d, a, b);
			TEST_ASSERT(bn_cmp(e, c) == CMP_EQ, end);
			bn_mul(e, c, b);
			bn_add(e, e, d);
			TEST_ASSERT(bn_cmp(a, e) == CMP_EQ, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	return code;
}

int reduction(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);

#if BN_MOD == BARRT || !defined(STRIP)
		TEST_BEGIN("barrett reduction is correct") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_div_basic(c, d, a, b);
			bn_sqr(c, b);
			if (bn_cmp(a, c) == CMP_LT) {
				bn_mod_barrt_setup(e, b);
				bn_mod_barrt(e, a, b, e);
				TEST_ASSERT(bn_cmp(e, d) == CMP_EQ, end);
			}
		}
		TEST_END;
#endif

#if (BN_MOD == MONTY && BN_MUL == BASIC) || !defined(STRIP)
		TEST_BEGIN("basic montgomery reduction is correct") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			if (bn_is_even(b)) {
				bn_add_dig(b, b, 1);
			}
			bn_mod_basic(a, a, b);
			bn_mod_monty_conv(c, a, b);
			bn_mod_monty_setup(e, b);
			bn_mod_monty_basic(d, c, b, e);
			TEST_ASSERT(bn_cmp(a, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if (BN_MOD == MONTY && BN_MUL == COMBA) || !defined(STRIP)
		TEST_BEGIN("comba montgomery reduction is correct") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			if (bn_is_even(b)) {
				bn_add_dig(b, b, 1);
			}
			bn_mod_basic(a, a, b);
			bn_mod_monty_conv(c, a, b);
			bn_mod_monty_setup(e, b);
			bn_mod_monty_comba(d, c, b, e);
			TEST_ASSERT(bn_cmp(a, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if BN_MOD == RADIX || !defined(STRIP)
		TEST_BEGIN("diminished radix reduction is correct") {
			bn_rand(a, BN_POS, BN_BITS - BN_DIGIT / 2);
			bn_rand(b, BN_POS, BN_BITS / 2);
			bn_rand(c, BN_POS, BN_DIGIT);
			bn_set_2b(b, BN_BITS / 2);
			if (bn_is_zero(c))
				bn_set_dig(c, 1);
			bn_sub(b, b, c);
			TEST_ASSERT(bn_mod_radix_check(b) == 1, end);
			bn_mod_basic(c, a, b);
			bn_mod_radix_setup(e, b);
			bn_mod_radix(d, a, b, e);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
#endif

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	return code;
}

int mxp(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, p = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(p);

#if BN_MOD != RADIX
		bn_gen_prime(p, BN_BITS);
#elif BN_PRECI >= 128
		/* Let's try a Mersenne prime. */
		char str[] = "170141183460469231731687303715884105727";
		bn_read_str(p, str, sizeof(str), 10);
#endif

		TEST_BEGIN("modular exponentiation is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_mod_basic(a, a, p);
			bn_copy(b, a);
#if BN_MOD == MONTY
			bn_mod_monty_conv(b, a, p);
#endif
			bn_mxp(b, b, p, p);
#if BN_MOD == MONTY
			bn_mod_monty_back(b, b, p);
#endif
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;

#if BN_MXP == BASIC || !defined(STRIP)
		TEST_BEGIN("basic modular exponentiation is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_mod_basic(a, a, p);
			bn_mxp_basic(b, a, p, p);
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if BN_MXP == SLIDE || !defined(STRIP)
		TEST_BEGIN("sliding window modular exponentiation is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_mod_basic(a, a, p);
			bn_copy(b, a);
#if BN_MOD == MONTY
			bn_mod_monty_conv(b, a, p);
#endif
			bn_mxp_slide(b, b, p, p);
#if BN_MOD == MONTY
			bn_mod_monty_back(b, b, p);
#endif
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;
#endif

#if BN_MXP == CONST || !defined(STRIP)
		TEST_BEGIN("constant time modular exponentiation is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_mod_basic(a, a, p);
			bn_copy(b, a);
#if BN_MOD == MONTY
			bn_mod_monty_conv(b, a, p);
#endif
			bn_mxp_const(b, b, p, p);
#if BN_MOD == MONTY
			bn_mod_monty_back(b, b, p);
#endif
			TEST_ASSERT(bn_cmp(a, b) == CMP_EQ, end);
		}
		TEST_END;
#endif

	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(p);
	return code;
}

int gcd(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL, f = NULL;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);
		bn_new(f);

		TEST_BEGIN("greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd(c, a, b);
			bn_mod_basic(d, a, c);
			bn_mod_basic(e, b, c);
			TEST_ASSERT(bn_is_zero(d) && bn_is_zero(e), end);
			bn_div_norem(a, a, c);
			bn_div_norem(b, b, c);
			bn_gcd(c, a, b);
			TEST_ASSERT(bn_cmp_dig(c, 1) == CMP_EQ, end);
			bn_rand(a, BN_POS, BN_BITS);
			bn_zero(b);
			bn_gcd(c, a, b);
			TEST_ASSERT(bn_cmp_abs(c, a) == CMP_EQ, end);
			bn_rand(a, BN_NEG, BN_BITS);
			bn_zero(b);
			bn_gcd(c, a, b);
			TEST_ASSERT(bn_cmp_abs(c, a) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("extended greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd_ext(c, d, e, a, b);
			bn_mul(d, d, a);
			bn_mul(e, e, b);
			bn_add(d, d, e);
			bn_gcd(f, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ && bn_cmp(c, f) == CMP_EQ, end);
		} TEST_END;

#if BN_GCD == BASIC || !defined(STRIP)
		TEST_BEGIN("basic greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd(c, a, b);
			bn_gcd_basic(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("basic extended greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd_ext_basic(c, d, e, a, b);
			bn_mul(d, d, a);
			bn_mul(e, e, b);
			bn_add(d, d, e);
			bn_gcd_basic(f, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ && bn_cmp(c, f) == CMP_EQ, end);
		} TEST_END;
#endif

#if BN_GCD == LEHME || !defined(STRIP)
		TEST_BEGIN("lehmer greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd(c, a, b);
			bn_gcd_lehme(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("lehmer extended greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd_ext_lehme(c, d, e, a, b);
			bn_mul(d, d, a);
			bn_mul(e, e, b);
			bn_add(d, d, e);
			bn_gcd_lehme(f, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ && bn_cmp(c, f) == CMP_EQ, end);
		} TEST_END;
#endif

#if BN_GCD == STEIN || !defined(STRIP)
		TEST_BEGIN("stein greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd(c, a, b);
			bn_gcd_stein(d, a, b);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("stein extended greatest common divisor is correct") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_BITS);
			bn_gcd_ext_stein(c, d, e, a, b);
			bn_mul(d, d, a);
			bn_mul(e, e, b);
			bn_add(d, d, e);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;
#endif
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	bn_free(f);
	return code;
}

int digit(void) {
	int code = STS_ERR;
	bn_t a = NULL, b = NULL, c = NULL, d = NULL, e = NULL, f = NULL;
	dig_t g;

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);
		bn_new(d);
		bn_new(e);
		bn_new(f);

		TEST_BEGIN("addition of a single digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&g, b);
			bn_add(c, a, b);
			bn_add_dig(d, a, g);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("subtraction of a single digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&g, b);
			bn_sub(c, a, b);
			bn_sub_dig(d, a, g);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("multiplication by a single digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&g, b);
			bn_mul(c, a, b);
			bn_mul_dig(d, a, g);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		} TEST_END;

		TEST_BEGIN("division by a single digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			if (b->dp[0] == 0)
				continue;
			bn_div_basic(d, c, a, b);
			bn_div_dig(e, &g, a, b->dp[0]);
			TEST_ASSERT(bn_cmp(d, e) == CMP_EQ, end);
			TEST_ASSERT(bn_cmp_dig(c, g) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("modular reduction modulo a digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			if (b->dp[0] == 0)
				continue;
			bn_div_basic(d, c, a, b);
			bn_mod_dig(&g, a, b->dp[0]);
			TEST_ASSERT(bn_cmp_dig(c, g) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN("greatest common divisor with a digit is consistent") {
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&g, b);
			bn_gcd(c, a, b);
			bn_gcd_dig(e, a, g);
			TEST_ASSERT(bn_cmp(c, e) == CMP_EQ, end);
		}
		TEST_END;

		TEST_BEGIN
				("extended greatest common divisor with a digit is consistent")
		{
			bn_rand(a, BN_POS, BN_BITS);
			bn_rand(b, BN_POS, BN_DIGIT);
			bn_get_dig(&g, b);
			bn_gcd_ext_dig(c, d, e, a, g);
			bn_mul(d, d, a);
			bn_mul(e, e, b);
			bn_add(d, d, e);
			TEST_ASSERT(bn_cmp(c, d) == CMP_EQ, end);
		}
		TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(a);
	bn_free(b);
	bn_free(c);
	bn_free(d);
	bn_free(e);
	bn_free(f);
	return code;
}

#undef TESTS
#define TESTS 1

int prime(void) {
	int code = STS_ERR;

	bn_t p;

	TRY {
		bn_new(p);

		TEST_BEGIN("prime generation is consistent") {
			bn_gen_prime(p, BN_BITS);
			TEST_ASSERT(bn_is_prime(p) == 1, end);
		} TEST_END;

#if BN_GEN == BASIC || !defined(STRIP)
		TEST_BEGIN("basic prime generation is consistent") {
			bn_gen_prime_basic(p, BN_BITS);
			TEST_ASSERT(bn_is_prime(p) == 1, end);
		} TEST_END;
#endif

#if BN_GEN == SAFEP || !defined(STRIP)
		TEST_BEGIN("safe prime generation is consistent") {
			bn_gen_prime_safep(p, BN_BITS);
			TEST_ASSERT(bn_is_prime(p) == 1, end);
			bn_sub_dig(p, p, 1);
			bn_hlv(p, p);
			TEST_ASSERT(bn_is_prime(p) == 1, end);
		} TEST_END;
#endif

#if BN_GEN == STRON || !defined(STRIP)
		TEST_BEGIN("strong prime generation is consistent") {
			bn_gen_prime_stron(p, BN_BITS);
			TEST_ASSERT(bn_is_prime(p) == 1, end);
		} TEST_END;
#endif

		TEST_BEGIN("basic prime testing is correct") {
			bn_gen_prime(p, BN_BITS);
			TEST_ASSERT(bn_is_prime_basic(p) == 1, end);
		} TEST_END;

		TEST_BEGIN("miller-rabin prime testing is correct") {
			bn_gen_prime(p, BN_BITS);
			TEST_ASSERT(bn_is_prime_rabin(p) == 1, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;
  end:
	bn_free(p);
	return code;
}

int main(void) {
	core_init();

	util_print_banner("Tests for the BN module", 0);
	util_print_banner("Utilities:", 1);

	if (memory() != STS_OK) {
		core_clean();
		return 1;
	}

	if (util() != STS_OK) {
		core_clean();
		return 1;
	}

	util_print_banner("Arithmetic:", 1);

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

	if (division() != STS_OK) {
		core_clean();
		return 1;
	}

	if (reduction() != STS_OK) {
		core_clean();
		return 1;
	}

	if (mxp() != STS_OK) {
		core_clean();
		return 1;
	}

	if (gcd() != STS_OK) {
		core_clean();
		return 1;
	}

	if (digit() != STS_OK) {
		core_clean();
		return 1;
	}

	if (prime() != STS_OK) {
		core_clean();
		return 1;
	}

	core_clean();
	return 0;
}
