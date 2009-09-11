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
	rlc_init();
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
	char *xq = "11A9EC4F973C447F88CCC08D63A022E15A4BAD7E064B1DB0C79DC7A0689A2FAB";
	char *yq = "20E82E4DA716661449A4C276D1A7314CF525EB0B3F1EC4A83C2EF6AA92D4B6FE";
	char *xp0 = "12BEDC036909BAC53D83FD2BC2E7C31DBD02FF4EADB571C0D6B277B8A7128A31";
	char *xp1 = "138F2DCFC297C09DA8397A82ACE6BF909068FF94A091B74C556C9238CE7196E5";
	char *yp0 = "2167A9677886B2A5CA59772C61D9338A06CA035B544029794FF820A4FA049F31";
	char *yp1 = "B9D45158AFE6311CC4D8E416827DBD53CBE5C5C0B7820BFB03BE9D083F92E1C";
	char *sx = "-4080000000000001";
	char *f0 = "9EBEE691ED1837503EAB22F57B96AC8DC178B6DB2C08850C582193F90D5922A";
	char *f1 = "1B377619212E7C8CB6499B50A846953F850974924D3F77C2E17DE6C06F2A6DE9";

	fp_read(q->x, xq, strlen(xq), 16);
	fp_read(q->y, yq, strlen(xq), 16);
	fp_set_dig(q->z, 1);
	fp_read(p->x[0], xp0, strlen(xp0), 16);
	fp_read(p->x[1], xp1, strlen(xp1), 16);
	fp_read(p->y[0], yp0, strlen(yp0), 16);
	fp_read(p->y[1], yp1, strlen(yp1), 16);
	fp12_zero(f);
	fp_read(f[1][0][0], f0, strlen(f0), 16);
	fp_read(f[1][0][1], f1, strlen(f1), 16);
	fp2_set_dig(p->z, 1);
	bn_read_str(x, sx, strlen(sx), 16);

	TEST_BEGIN("rate pairing is bilinear") {
		ep_dbl(t, q);
		pp_pair_rate2(r1, p, t, x, f);
		ep2_dbl(p, p);
		ep2_norm(p, p);
		pp_pair_rate(r2, p, q, x, f);
		TEST_ASSERT(fp12_cmp(r1, r2) == RLC_EQ, end);
	} TEST_END;

	BENCH_BEGIN("rate_pair") {
		BENCH_ADD(pp_pair_rate2(r1, p, q, x, f));
	} BENCH_END;

	BENCH_BEGIN("rate_pair") {
		BENCH_ADD(pp_pair_rate(r1, p, q, x, f));
	} BENCH_END;

	ep2_free(p);
	ep_free(q);
	ep_free(t);
	bn_free(x);
	fp12_free(r1);
	fp12_free(r2);
	fp12_free(f);
end:
	rlc_clean();

	return 0;
}
