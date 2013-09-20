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

#include "relic_dv_low.h"

#include "macro.s"

.text

.global fp2_addn_low
.global fp2_addm_low
.global fp2_addd_low
.global fp2_addc_low
.global fp2_subn_low
.global fp2_subm_low
.global fp2_subd_low
.global fp2_subc_low
.global fp2_dbln_low
.global fp2_dblm_low
.global fp2_norm_low
.global fp2_nord_low

/*
 * Function: fp2_addn_low
 * Inputs: rdi = c, rsi = a, rdx = b
 * Output: rax
 */
fp2_addn_low:
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	ADDN_STEP 1 (FP_DIGS - 1)

	addq	$(8*FP_DIGS), %rdx
	addq	$(8*FP_DIGS), %rsi
	addq	$(8*FP_DIGS), %rdi
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	ADDN_STEP 1 (FP_DIGS - 1)

	ret

fp2_addm_low:
	push	%r12
	push	%r13
	push	%r14
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	8(%rdx), %r9
	adcq	8(%rsi), %r9
	movq	16(%rdx), %r10
	adcq	16(%rsi), %r10
	movq	24(%rdx), %r11
	adcq	24(%rsi), %r11

	xorq	%rax, %rax
	movq 	%r8, %rax
	movq 	%r9, %rcx
	movq 	%r10, %r13
	movq 	%r11, %r14

	movq	P0, %r12
	subq	%r12, %rax
	movq	P1, %r12
	sbbq	%r12, %rcx
	movq	P2, %r12
	sbbq	%r12, %r13
	movq	P3, %r12
	sbbq	%r12, %r14

	cmovnc	%rax, %r8
	cmovnc	%rcx, %r9
	cmovnc	%r13, %r10
	cmovnc	%r14, %r11

	movq	%r8,0(%rdi)
	movq	%r9,8(%rdi)
	movq	%r10,16(%rdi)
	movq	%r11,24(%rdi)

	addq	$(8*FP_DIGS), %rdx
	addq	$(8*FP_DIGS), %rsi
	addq	$(8*FP_DIGS), %rdi
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	8(%rdx), %r9
	adcq	8(%rsi), %r9
	movq	16(%rdx), %r10
	adcq	16(%rsi), %r10
	movq	24(%rdx), %r11
	adcq	24(%rsi), %r11

	xorq	%rax, %rax
	movq 	%r8, %rax
	movq 	%r9, %rcx
	movq 	%r10, %r13
	movq 	%r11, %r14

	movq	P0, %r12
	subq	%r12, %rax
	movq	P1, %r12
	sbbq	%r12, %rcx
	movq	P2, %r12
	sbbq	%r12, %r13
	movq	P3, %r12
	sbbq	%r12, %r14

	cmovnc	%rax, %r8
	cmovnc	%rcx, %r9
	cmovnc	%r13, %r10
	cmovnc	%r14, %r11

	movq	%r8,0(%rdi)
	movq	%r9,8(%rdi)
	movq	%r10,16(%rdi)
	movq	%r11,24(%rdi)

	pop		%r14
	pop		%r13
	pop		%r12
	ret

fp2_addd_low:
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	ADDN_STEP 1 (2 * FP_DIGS - 1)

	addq	$(8*DV_DIGS), %rdx
	addq	$(8*DV_DIGS), %rsi
	addq	$(8*DV_DIGS), %rdi
	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	ADDN_STEP 1 (2 * FP_DIGS - 1)

	ret

fp2_addc_low:
	push    %r12
	push    %r13
	push    %r14
	movq    0(%rsi), %r8
	addq    0(%rdx), %r8
	movq    %r8,0(%rdi)
	movq    8(%rsi), %r8
	adcq    8(%rdx), %r8
	movq    %r8,8(%rdi)
	movq    16(%rsi), %r8
	adcq    16(%rdx), %r8
	movq    %r8,16(%rdi)
	movq    24(%rsi), %r8
	adcq    24(%rdx), %r8
	movq    %r8,24(%rdi)
	movq    32(%rsi), %r8
	adcq    32(%rdx), %r8
	movq    %r8,%r12
	movq    40(%rsi), %r9
	adcq    40(%rdx), %r9
	movq    %r9,%r13
	movq    48(%rsi), %r10
	adcq    48(%rdx), %r10
	movq    %r10,%r14
	movq    56(%rsi), %r11
	adcq    56(%rdx), %r11
	movq    %r11,%rcx

	movq    P0,%rax
	subq    %rax,%r8
	movq    P1,%rax
	sbbq    %rax,%r9
	movq    P2,%rax
	sbbq    %rax,%r10
	movq    P3,%rax
	sbbq    %rax,%r11
    cmovc   %r12, %r8
    cmovc   %r13, %r9
    cmovc   %r14, %r10
    cmovc   %rcx, %r11
	movq    %r8,32(%rdi)
	movq    %r9,40(%rdi)
	movq    %r10,48(%rdi)
	movq    %r11,56(%rdi)

	addq	$(8*DV_DIGS), %rdx
	addq	$(8*DV_DIGS), %rsi
	addq	$(8*DV_DIGS), %rdi

	movq    0(%rsi), %r8
	addq    0(%rdx), %r8
	movq    %r8,0(%rdi)
	movq    8(%rsi), %r8
	adcq    8(%rdx), %r8
	movq    %r8,8(%rdi)
	movq    16(%rsi), %r8
	adcq    16(%rdx), %r8
	movq    %r8,16(%rdi)
	movq    24(%rsi), %r8
	adcq    24(%rdx), %r8
	movq    %r8,24(%rdi)
	movq    32(%rsi), %r8
	adcq    32(%rdx), %r8
	movq    %r8,%r12
	movq    40(%rsi), %r9
	adcq    40(%rdx), %r9
	movq    %r9,%r13
	movq    48(%rsi), %r10
	adcq    48(%rdx), %r10
	movq    %r10,%r14
	movq    56(%rsi), %r11
	adcq    56(%rdx), %r11
	movq    %r11,%rcx

	movq    P0,%rax
	subq    %rax,%r8
	movq    P1,%rax
	sbbq    %rax,%r9
	movq    P2,%rax
	sbbq    %rax,%r10
	movq    P3,%rax
	sbbq    %rax,%r11
    cmovc   %r12, %r8
    cmovc   %r13, %r9
    cmovc   %r14, %r10
    cmovc   %rcx, %r11
	movq    %r8,32(%rdi)
	movq    %r9,40(%rdi)
	movq    %r10,48(%rdi)
	movq    %r11,56(%rdi)

	pop	%r14
	pop	%r13
	pop	%r12
    ret

fp2_subn_low:
	movq	0(%rsi), %r8
	subq	0(%rdx), %r8
	movq	%r8,0(%rdi)
	movq	8(%rsi), %r9
	sbbq	8(%rdx), %r9
	movq	%r9,8(%rdi)
	movq	16(%rsi), %r10
	sbbq	16(%rdx), %r10
	movq	%r10,16(%rdi)
	movq	24(%rsi), %r11
	sbbq	24(%rdx), %r11
	movq	%r11,24(%rdi)

	movq	$0, %r12
	movq	$0, %r13
	movq	P0,%r8
	movq	P1,%r9
	movq	P2,%r10
	movq	P3,%r11
	cmovc	%r8, %rax
	cmovc	%r9, %rcx
	cmovc	%r10, %r12
	cmovc	%r11, %r13
    addq	%rax,0(%rdi)
    adcq	%rcx,8(%rdi)
    adcq	%r12,16(%rdi)
    adcq	%r13,24(%rdi)

	addq	$(8*FP_DIGS), %rdx
	addq	$(8*FP_DIGS), %rsi
	addq	$(8*FP_DIGS), %rdi

	xorq	%rax,%rax
	xorq	%rcx,%rcx

	movq	0(%rsi), %r8
	subq	0(%rdx), %r8
	movq	%r8,0(%rdi)
	movq	8(%rsi), %r9
	sbbq	8(%rdx), %r9
	movq	%r9,8(%rdi)
	movq	16(%rsi), %r10
	sbbq	16(%rdx), %r10
	movq	%r10,16(%rdi)
	movq	24(%rsi), %r11
	sbbq	24(%rdx), %r11
	movq	%r11,24(%rdi)

	movq	$0, %r12
	movq	$0, %r13
	movq	P0,%r8
	movq	P1,%r9
	movq	P2,%r10
	movq	P3,%r11
	cmovc	%r8, %rax
	cmovc	%r9, %rcx
	cmovc	%r10, %r12
	cmovc	%r11, %r13
    addq	%rax,0(%rdi)
    adcq	%rcx,8(%rdi)
    adcq	%r12,16(%rdi)
    adcq	%r13,24(%rdi)

	ret

fp2_subm_low:
	push	%r12
	push	%r13
	xorq	%rax,%rax
	xorq	%rcx,%rcx

	movq	0(%rsi), %r8
	subq	0(%rdx), %r8
	movq	%r8,0(%rdi)
	movq	8(%rsi), %r9
	sbbq	8(%rdx), %r9
	movq	%r9,8(%rdi)
	movq	16(%rsi), %r10
	sbbq	16(%rdx), %r10
	movq	%r10,16(%rdi)
	movq	24(%rsi), %r11
	sbbq	24(%rdx), %r11
	movq	%r11,24(%rdi)

	movq	$0, %r12
	movq	$0, %r13
	movq	P0,%r8
	movq	P1,%r9
	movq	P2,%r10
	movq	P3,%r11
	cmovc	%r8, %rax
	cmovc	%r9, %rcx
	cmovc	%r10, %r12
	cmovc	%r11, %r13
    addq	%rax,0(%rdi)
    adcq	%rcx,8(%rdi)
    adcq	%r12,16(%rdi)
    adcq	%r13,24(%rdi)

	xorq	%rax,%rax
	xorq	%rcx,%rcx

	addq	$(8*FP_DIGS), %rdx
	addq	$(8*FP_DIGS), %rsi
	addq	$(8*FP_DIGS), %rdi

	movq	0(%rsi), %r8
	subq	0(%rdx), %r8
	movq	%r8,0(%rdi)
	movq	8(%rsi), %r9
	sbbq	8(%rdx), %r9
	movq	%r9,8(%rdi)
	movq	16(%rsi), %r10
	sbbq	16(%rdx), %r10
	movq	%r10,16(%rdi)
	movq	24(%rsi), %r11
	sbbq	24(%rdx), %r11
	movq	%r11,24(%rdi)

	movq	$0, %r12
	movq	$0, %r13
	movq	P0,%r8
	movq	P1,%r9
	movq	P2,%r10
	movq	P3,%r11
	cmovc	%r8, %rax
	cmovc	%r9, %rcx
	cmovc	%r10, %r12
	cmovc	%r11, %r13
    addq	%rax,0(%rdi)
    adcq	%rcx,8(%rdi)
    adcq	%r12,16(%rdi)
    adcq	%r13,24(%rdi)

    pop		%r13
    pop		%r12
	ret

fp2_subd_low:
	movq	0(%rdx), %r8
	subq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	SUBN_STEP 1 (2 * FP_DIGS - 1)

	addq	$(8*DV_DIGS), %rdx
	addq	$(8*DV_DIGS), %rsi
	addq	$(8*DV_DIGS), %rdi

	movq	0(%rdx), %r8
	addq	0(%rsi), %r8
	movq	%r8, 0(%rdi)

	SUBN_STEP 1 (2 * FP_DIGS - 1)

	ret

fp2_subc_low:
	push	%r12
	push	%r13
	xorq    %rax, %rax
	xorq    %rcx, %rcx
	xorq    %r13, %r13
	movq    0(%rsi), %r8
	subq    0(%rdx), %r8
	movq    %r8, 0(%rdi)

	SUBN_STEP 1 (2 * FP_DIGS-1)

	movq	$0, %r8
	movq    P0, %r9
	movq    P1, %r10
	movq    P2, %r11
	movq    P3, %r12
	cmovc   %r9, %rax
	cmovc   %r10, %rcx
	cmovc   %r11, %r8
	cmovc   %r12, %r13
	addq    %rax, 32(%rdi)
	adcq    %rcx, 40(%rdi)
	adcq    %r8, 48(%rdi)
	adcq    %r13, 56(%rdi)

	xorq    %rax,%rax
	xorq    %rcx,%rcx
	xorq    %r13,%r13

	addq	$(8*DV_DIGS), %rdx
	addq	$(8*DV_DIGS), %rsi
	addq	$(8*DV_DIGS), %rdi

	movq    0(%rsi), %r8
	subq    0(%rdx), %r8
	movq    %r8, 0(%rdi)

	SUBN_STEP 1 (2 * FP_DIGS-1)

	movq	$0, %r8
	cmovc   %r9, %rax
	cmovc   %r10, %rcx
	cmovc   %r11, %r8
	cmovc   %r12, %r13
	addq    %rax, 32(%rdi)
	adcq    %rcx, 40(%rdi)
	adcq    %r8, 48(%rdi)
	adcq    %r13, 56(%rdi)
	pop	%r13
	pop	%r12
	ret

fp2_dblm_low:
	push	%r12
	push	%r13
	push	%r14
	movq	0(%rsi), %r8
	addq	%r8, %r8
	movq	8(%rsi), %r9
	adcq	%r9, %r9
	movq	16(%rsi), %r10
	adcq	%r10, %r10
	movq	24(%rsi), %r11
	adcq	%r11, %r11
    adcq	%rax,%rax

	xorq	%rax, %rax
	movq 	%r8, %rax
	movq 	%r9, %rcx
	movq 	%r10, %r13
	movq 	%r11, %r14

	movq	P0, %r12
	subq	%r12, %rax
	movq	P1, %r12
	sbbq	%r12, %rcx
	movq	P2, %r12
	sbbq	%r12, %r13
	movq	P3, %r12
	sbbq	%r12, %r14

	cmovnc	%rax, %r8
	cmovnc	%rcx, %r9
	cmovnc	%r13, %r10
	cmovnc	%r14, %r11

	movq	%r8,0(%rdi)
	movq	%r9,8(%rdi)
	movq	%r10,16(%rdi)
	movq	%r11,24(%rdi)

	xorq	%rax,%rax
	xorq	%rdx,%rdx

	addq	$(8*FP_DIGS), %rdx
	addq	$(8*FP_DIGS), %rsi
	addq	$(8*FP_DIGS), %rdi

	movq	0(%rsi), %r8
	addq	%r8, %r8
	movq	8(%rsi), %r9
	adcq	%r9, %r9
	movq	16(%rsi), %r10
	adcq	%r10, %r10
	movq	24(%rsi), %r11
	adcq	%r11, %r11
    adcq	%rax,%rax

	xorq	%rax, %rax
	movq 	%r8, %rax
	movq 	%r9, %rcx
	movq 	%r10, %r13
	movq 	%r11, %r14

	movq	P0, %r12
	subq	%r12, %rax
	movq	P1, %r12
	sbbq	%r12, %rcx
	movq	P2, %r12
	sbbq	%r12, %r13
	movq	P3, %r12
	sbbq	%r12, %r14

	cmovnc	%rax, %r8
	cmovnc	%rcx, %r9
	cmovnc	%r13, %r10
	cmovnc	%r14, %r11

	movq	%r8,0(%rdi)
	movq	%r9,8(%rdi)
	movq	%r10,16(%rdi)
	movq	%r11,24(%rdi)

	pop		%r14
	pop		%r13
	pop		%r12
	ret

fp2_norm_low:
	push	%r12
	push	%r13
	xorq	%rax, %rax
	xorq	%rcx, %rcx
	movq	0(%rsi), %r8
	subq	8*FP_DIGS(%rsi), %r8
	movq	%r8, 0(%rdi)
	movq	8(%rsi), %r8
	sbbq	8*FP_DIGS+8(%rsi), %r8
	movq	%r8, 8(%rdi)
	movq	16(%rsi), %r8
	sbbq	8*FP_DIGS+16(%rsi), %r8
	movq	%r8, 16(%rdi)
	movq	24(%rsi), %r8
	sbbq	8*FP_DIGS+24(%rsi), %r8
	movq	%r8, 24(%rdi)

	movq	$0, %r12
	movq	$0, %r13
	movq	P0, %r8
	movq	P1, %r9
	movq	P2, %r10
	movq	P3, %r11
	cmovc	%r8, %rax
	cmovc	%r9, %rcx
	cmovc	%r10, %r12
	cmovc	%r11, %r13
	addq	%rax, 0(%rdi)
	adcq	%rcx, 8(%rdi)
	adcq	%r12, 16(%rdi)
	adcq	%r13, 24(%rdi)

	xorq	%rax, %rax
	movq	0(%rsi), %r8
	addq	8*FP_DIGS(%rsi), %r8
	movq	8(%rsi), %r9
	adcq	8*FP_DIGS+8(%rsi), %r9
	movq	16(%rsi), %r10
	adcq	8*FP_DIGS+16(%rsi), %r10
	movq	24(%rsi), %r11
	adcq	8*FP_DIGS+24(%rsi), %r11

	movq 	%r8, %rax
	movq 	%r9, %rcx
	movq 	%r10, %r12
	movq 	%r11, %r13

	movq	P0, %rdx
	subq	%rdx, %rax
	movq	P1, %rdx
	sbbq	%rdx, %rcx
	movq	P2, %rdx
	sbbq	%rdx, %r12
	movq	P3, %rdx
	sbbq	%rdx, %r13

	cmovnc	%rax, %r8
	cmovnc	%rcx, %r9
	cmovnc	%r12, %r10
	cmovnc	%r13, %r11

	movq	%r8, 8*FP_DIGS(%rdi)
	movq	%r9, 8*FP_DIGS+8(%rdi)
	movq	%r10, 8*FP_DIGS+16(%rdi)
	movq	%r11, 8*FP_DIGS+24(%rdi)
	xorq	%rax, %rax
	pop	%r13
	pop	%r12

	ret

fp2_nord_low:
	push	%r12
	xorq    %rax, %rax
	xorq    %rcx, %rcx
	xorq    %rdx, %rdx
	movq	0(%rsi), %r8
	subq	8*DV_DIGS(%rsi), %r8
	movq	%r8, 0(%rdi)
	movq	8(%rsi), %r8
	sbbq	8*DV_DIGS+8(%rsi), %r8
	movq	%r8, 8(%rdi)
	movq	16(%rsi), %r8
	sbbq	8*DV_DIGS+16(%rsi), %r8
	movq	%r8, 16(%rdi)
	movq	24(%rsi), %r8
	sbbq	8*DV_DIGS+24(%rsi), %r8
	movq	%r8, 24(%rdi)
	movq	32(%rsi), %r8
	sbbq	8*DV_DIGS+32(%rsi), %r8
	movq	%r8, 32(%rdi)
	movq	40(%rsi), %r8
	sbbq	8*DV_DIGS+40(%rsi), %r8
	movq	%r8, 40(%rdi)
	movq	48(%rsi), %r8
	sbbq	8*DV_DIGS+48(%rsi), %r8
	movq	%r8, 48(%rdi)
	movq	56(%rsi), %r8
	sbbq	8*DV_DIGS+56(%rsi), %r8
	movq	%r8, 56(%rdi)

	movq	$0, %r8
	movq    P0, %r9
	movq    P1, %r10
	movq    P2, %r11
	movq    P3, %r12
	cmovc   %r9, %r8
	cmovc   %r10, %rax
	cmovc   %r11, %rcx
	cmovc   %r12, %rdx
	addq    %r8, 32(%rdi)
	adcq    %rax, 40(%rdi)
	adcq    %rcx, 48(%rdi)
	adcq    %rdx, 56(%rdi)

	xorq    %rax, %rax
	movq    0(%rsi), %r8
	addq    8*DV_DIGS(%rsi), %r8
	movq    %r8, 8*DV_DIGS(%rdi)
	movq    8(%rsi), %r8
	adcq    8*DV_DIGS+8(%rsi), %r8
	movq    %r8, 8*DV_DIGS+8(%rdi)
	movq    16(%rsi), %r8
	adcq    8*DV_DIGS+16(%rsi), %r8
	movq    %r8, 8*DV_DIGS+16(%rdi)
	movq    24(%rsi), %r8
	adcq    8*DV_DIGS+24(%rsi), %r8
	movq    %r8, 8*DV_DIGS+24(%rdi)
	movq    32(%rsi), %r8
	adcq    8*DV_DIGS+32(%rsi), %r8
	movq    %r8, %r12
	movq    40(%rsi), %r9
	adcq    8*DV_DIGS+40(%rsi), %r9
	movq    %r9, %rax
	movq    48(%rsi), %r10
	adcq    8*DV_DIGS+48(%rsi), %r10
	movq    %r10, %rcx
	movq    56(%rsi), %r11
	adcq    8*DV_DIGS+56(%rsi), %r11
	movq    %r11, %rdx

	movq    P0, %rsi
	subq    %rsi, %r8
	movq    P1, %rsi
	sbbq    %rsi, %r9
	movq    P2, %rsi
	sbbq    %rsi, %r10
	movq    P3, %rsi
	sbbq    %rsi, %r11
    cmovc   %r12, %r8
    cmovc   %rax, %r9
    cmovc   %rcx, %r10
    cmovc   %rdx, %r11
	movq    %r8, 8*DV_DIGS+32(%rdi)
	movq    %r9, 8*DV_DIGS+40(%rdi)
	movq    %r10, 8*DV_DIGS+48(%rdi)
	movq    %r11, 8*DV_DIGS+56(%rdi)
	xorq    %rax, %rax
	pop	%r12

	ret
