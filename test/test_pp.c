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
 * Tests for the quartic extension binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

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

	ep_param_set(RATE_P256);
	ep2_curve_set_twist(1);
	char *xq = "1";
	char *yq = "C7424FC261B627189A14E3433B4713E9C2413FCF89B8E2B178FB6322EFB2AB3";
	char *xp0 = "1822AA754FAFAFF95FE37842D7D5DECE88305EC19B363F6681DF06BF405F02B4";
	char *xp1 = "1AB4CC8A133A7AA970AADAE37C20D1C7191279CBA02830AFC64C19B50E8B1997";
	char *yp0 = "16737CF6F9DEC5895A7E5A6D60316763FB6638A0A82F26888E909DA86F7F84BA";
	char *yp1 = "5B6DB6FF5132FB917E505627E7CCC12E0CE9FCC4A59805B3B730EE0EC44E43C";
	char *sx = "-4080000000000001";
	char *f0 = "1B377619212E7C8CB6499B50A846953F850974924D3F77C2E17DE6C06F2A6DE9";
	char *f1 = "9EBEE691ED1837503EAB22F57B96AC8DC178B6DB2C08850C582193F90D5922A";

	fp_read(q->x, xq, strlen(xq), 16);
	fp_read(q->y, yq, strlen(yq), 16);
	fp_set_dig(q->z, 1);
	fp_read(p->x[0], xp0, strlen(xp0), 16);
	fp_read(p->x[1], xp1, strlen(xp1), 16);
	fp_read(p->y[0], yp0, strlen(yp0), 16);
	fp_read(p->y[1], yp1, strlen(yp1), 16);
	fp2_set_dig(p->z, 1);
	fp12_zero(f);
	fp_read(f[1][0][0], f0, strlen(f0), 16);
	fp_read(f[1][0][1], f1, strlen(f1), 16);
	bn_read_str(x, sx, strlen(sx), 16);

	TEST_BEGIN("rate pairing is bilinear") {
		ep_dbl(t, q);
		pp_pair_rate4(r1, p, t, x, f);
		ep2_dbl(p, p);
		ep2_norm(p, p);
		pp_pair_rate4(r2, p, q, x, f);
		TEST_ASSERT(fp12_cmp(r1, r2) == CMP_EQ, end);
	} TEST_END;

//	BENCH_BEGIN("rate_pair") {
//		BENCH_ADD(pp_pair_rate2(r1, p, q, x, f));
//	} BENCH_END;
//
//	BENCH_BEGIN("rate_pair") {
//		BENCH_ADD(pp_pair_rate(r1, p, q, x, f));
//	} BENCH_END;

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
