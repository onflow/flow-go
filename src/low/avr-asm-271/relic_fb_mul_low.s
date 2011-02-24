/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009, 2010 RELIC Authors
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
 * Implementation of the low-level binary field bit shifting functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_fb_low.h"

.arch atmega128

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/*
 * Pointer register which points to the parameter a.
 */
#define A_PTR		X

/*
 * Pointer register which points to the parameter b.
 */
#define B_PTR		Z

/*
 * Pointer register which points to the parameter c.
 */
#define C_PTR		Z

/*
 * Temporary register.
 */
#define RT			r23

/*
 * Carry register for shifting.
 */
#define RC			r22

#define T_LINE 24
#undef RT
#undef RC
#define RT	r25
#define RC	r24

/*
 * Size of the multiplication precomputation table in bytes.
 */
#define T_DIGS	(16 * T_LINE)

/*
 * Size of the precomputation table in blocks of 256 bytes.
 */
#if (T_DIGS % 256) > 0
#define T_BLOCKS	(T_DIGS/256 + 1)
#else
#define T_BLOCKS	(T_DIGS/256)
#endif

/*
 * Precomputation table stored on a custom section (address must be terminated
 * in 0x00).
 */
.data
.byte
fb_muln_table0: .space 256, 0
.if T_BLOCKS > 1
fb_muln_table1: .space 256, 0
#if FB_POLYN == 271
fb_muln_table2: .space 256, 0
#endif
fb_sqrt_table:
.byte 0x00, 0x01, 0x04, 0x05, 0x10, 0x11, 0x14, 0x15, 0x40, 0x41, 0x44, 0x45, 0x50, 0x51, 0x54, 0x55
.endif

.global fb_sqrt_table


/*
 * Loads the pointer Y with the 16-bit address composed by i and j.
 */
.macro LOAD_T_PTR i j
	ldi		r28, \i
	ldi		r29, \j
.endm

/*
 * Prepares the first column values.
 */
.macro PREP_FIRST r1, r2, r4, r8
	ldd		\r1, B_PTR + 0	; r18 = r1 = b[0]
	mov		\r2, \r1
	lsl		\r2				; r19 = r2 = r1 << 1
	mov		\r4, \r2
	lsl		\r4				; r20 = r4 = r1 << 2
	mov 	\r8, \r4
	lsl		\r8				; r21 = r8 = r1 << 3
.endm

/*
 * Prepares the values for column i.
 */
.macro PREP_COLUMN i, r1, r2, r4, r8
	mov		RT, \r1

	ldd		\r1, B_PTR+\i	; r1 = b[i]
	mov		\r2, \r1
	lsl		RT
	rol		\r2
	mov		\r4, \r2
	lsl 	RT
	rol		\r4
	mov		\r8, \r4
	lsl		RT
	rol		\r8
.endm

/*
 * Prepares the values for the last column.
 */
.macro PREP_LAST i, r1, r2, r4, r8
	swap	\r1
	andi	\r1, 0x0f
	lsr		\r1			; t = b[i] >> 5

	mov		\r8, \r1
	lsr		\r1
	mov		\r4, \r1
	lsr		\r1
	mov		\r2, \r1
	clr		\r1
.endm

/*
 * Fills a column of the precomputation table.
 */
.macro FILL_COLUMN offset, r1, r2, r4, r8
	std		y + 00, \r1		; tab[1][i] = r1
	std		y + 16, \r2		; tab[2][i] = r2;
	mov		RT, \r1
	eor		RT, \r2
	std		y + 32, RT		; tab[3][i] = r1^r2
	std		y + 48, \r4		; tab[4][i] = r4
	add		r28, RC			; &tab[5][i]
	eor		RT, \r4
	std		y + 32, RT		; tab[7][i] = r1^r2^r4
	eor		RT, \r1
	std		y + 16, RT		; tab[6][i] = r2^r4
	mov		RT, \r1
	eor		RT, \r4
	std		y + 00, RT		; tab[5][i] = r1^r4;
	std		y + 48, \r8		; tab[8][i] = r8
	add		r28, RC			; &tab[9][i]
	mov		RT, \r1
	eor		RT, \r8			;
	std		y + 00, RT		; tab[9][i] = r1^r8
	eor		RT, \r2
	std		y + 32, RT		; tab[11][i] = r1^r2^r8
	eor		RT, \r1
	std		y + 16, RT		; tab[10][i] = r2^r8
	eor		\r4, \r8
	std		y + 48, \r4		; tab[12][i] = r4^r8
	add		r28, RC			; &tab[13][i]
	eor		\r4, \r1
	std		y + 00, \r4		; tab[13][i] = r1^r4^r8
	eor		\r4, \r2
	std		y + 32, \r4		; tab[15][i] = r1^r2^r4^r8
	eor		\r4, \r1
	std		y + 16, \r4		; tab[14][i] = r2^r4^r8
.endm

.macro FILL_LAST offset r1, r2, r4, r8
	;std		y + 00, \r1		; tab[1][i] = r1
	std		y + 16, \r2		; tab[2][i] = r2
	std		y + 32, \r2		; tab[3][i] = r2
	std		y + 48, \r4		; tab[4][i] = r4
	add		r28, RC			; &tab[5][i]
	std		y + 00,	\r4		; tab[5][i] = r1^r4
	mov		RT, \r2
	eor		RT, \r4			; r6 = r2^r4
	std		y + 16, RT		; tab[6][i] = r6
	std		y + 32, RT		; tab[7][i] = r1^r6;
	std		y + 48, \r8		; tab[8][i] = r8
	add		r28, RC			; &tab[9][i]
	std		y + 00,\r8		; tab[9][i] = r9
	eor		\r2, \r8
	std		y + 16,\r2		; tab[10][i] = r2^r8
	std		y + 32,\r2		; tab[11][i] = r2^r9
	eor		\r4, \r8
	std		y + 48,\r4		; tab[12][i] = r4^r8
	add		r28, RC			; &tab[13][i];
	std		y + 0, \r4		; tab[13][i] = r4^r9
	eor		RT, \r8
	std		y + 16, RT		; tab[14][i] = r6^r8
	std		y + 32, RT		; tab[15][i] = r6^r9
.endm

.macro FILL_TABLE i, j
	/*
	 * We do not need to initialize the first row with zeroes, as the .data
	 * section is already initialized with null bytes.
	 */
	.if \i == 0
		ldi		r28, 16
		PREP_FIRST	r18, r19, r20, r21
		FILL_COLUMN	16, r18, r19, r20, r21
	.else
		.if \i >= 16
			ldi		r28, \i
			PREP_COLUMN	\i, r18, r19, r20, r21
			FILL_COLUMN	\i, r18, r19, r20, r21
		.else
			ldi		r28, \i + 16
			PREP_COLUMN	\i, r18, r19, r20, r21
			FILL_COLUMN	\i + 16, r18, r19, r20, r21
		.endif
	.endif
	.if \i < \j
		FILL_TABLE \i + 1, \j
	.endif
.endm

.macro SHIFT i, offset
	swap	\i
	mov		RT, \i
	andi	RT, 0x0F
	eor		\i, RT
	eor		\i + 1, RT
	std		C_PTR + (\i + \offset + 1), \i + 1
.endm

.macro MUL0_STEP i
	.if \i == 16
		inc		r29
	.endif
	.if \i > 15
		ldd		\i, y + (\i - 16)
	.else
		ldd		\i, y + \i
	.endif
	.if \i < T_LINE - 1
		MUL0_STEP \i + 1
	.endif
.endm

.macro MUL_STEP i, j
	.if \j == 16
		inc		r29
	.endif
	.if \j == (T_LINE - 1)
		.if \j > 15
			ldd		\i, y + (\j - 16)
		.else
			ldd		\i, y + \j
		.endif
	.else
		.if \j > 15
			ldd		RT, y + (\j - 16)
		.else
			ldd		RT, y + \j
		.endif
		eor		\i, RT
	.endif
	.if \j < (T_LINE - 1)
		.if \i == (T_LINE - 1)
			MUL_STEP 0, \j + 1
		.else
			MUL_STEP \i + 1, \j + 1
		.endif
	.endif
.endm

.macro MULH i
	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldi		r29, hi8(fb_muln_table0)

	.if \i == 0
		MUL0_STEP 0

		swap	r0
		mov		RT, r0
		andi	RT, 0x0F
		mov		RC, RT
		eor		r0, RT

		std		C_PTR + 0, r0
	.else
		MUL_STEP \i, 0

		swap	\i
		mov		RT, \i
		andi	RT, 0x0F
		eor		\i, RT
		eor		\i, RC
		mov		RC, RT

		std		C_PTR + \i, \i
	.endif
.endm

.macro MULL i
	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldi		r29, hi8(fb_muln_table0)

	.if \i == 0
		MUL0_STEP 0
	.else
		MUL_STEP \i, 0
	.endif

	ldd		RT, C_PTR + \i
	eor		\i, RT
	std		C_PTR + \i, \i
.endm

.macro PROLOGUE
	.irp i, 0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 28, 29
		push 	\i
	.endr
.endm

.macro EPILOGUE
	clr		r1
	.irp i, 29, 28, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 0
		pop 	\i
	.endr
	ret
.endm

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.text

.global	fb_muln_low

#undef RT
#define RT	r25
#undef RC
#define RC	r24

.macro MULN_TABLE
	ldi		RC, 0x40
	ldi		r29, hi8(fb_muln_table0)
	FILL_TABLE 0, 15
	ldi		r29, hi8(fb_muln_table1)
	FILL_TABLE 16, 31
	ldi		r29, hi8(fb_muln_table2)
	ldi		r28, 16
	PREP_COLUMN 32, 18, 19, 20, 21
	FILL_COLUMN 32, 18, 19, 20, 21
	ldi		r28, 17
	PREP_COLUMN 33, 18, 19, 20, 21
	FILL_COLUMN 33, 18, 19, 20, 21
	ldi		r28, 18
	PREP_LAST 34, 18, 19, 20, 21
	FILL_LAST 34, 18, 19, 20, 21
.endm

.macro MULH_STEP10 i
	ld 		r28, A_PTR+
	andi	r28, 0xF0

	ldd		RT, y + 8
	eor		\i, RT
	ldd		RT, y + 9
	eor		\i + 1, RT
	ldd		RT, y + 10
	eor		\i + 2, RT
	ldd		RT, y + 11
	eor		\i + 3, RT
	ldd		RT, y + 12
	eor		\i + 4, RT
	ldd		RT, y + 13
	eor		\i + 5, RT
	ldd		RT, y + 14
	eor		\i + 6, RT
	ldd		RT, y + 15
	eor		\i + 7, RT

	inc r29

	ldd		RT, y + 0
	eor		\i + 8, RT
	ldd		RT, y + 1
	eor		\i + 9, RT
	ldd		RT, y + 2
	eor		\i + 10, RT

	dec r29
.endm

.macro MULH_STEPF i
	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldi		r29, hi8(fb_muln_table0)

	MUL_STEP \i, 0

	swap	\i
	mov		RT, \i
	andi	RT, 0x0F
	eor		\i, RT
	eor		\i, RC
	mov		RC, RT

	std		C_PTR + 24 + \i, \i
.endm

.macro MULL_STEP10 i
	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28

	ldd		RT, y + 8
	eor		\i, RT
	ldd		RT, y + 9
	eor		\i + 1, RT
	ldd		RT, y + 10
	eor		\i + 2, RT
	ldd		RT, y + 11
	eor		\i + 3, RT
	ldd		RT, y + 12
	eor		\i + 4, RT
	ldd		RT, y + 13
	eor		\i + 5, RT
	ldd		RT, y + 14
	eor		\i + 6, RT
	ldd		RT, y + 15
	eor		\i + 7, RT

	inc r29

	ldd		RT, y + 0
	eor		\i + 8, RT
	ldd		RT, y + 1
	eor		\i + 9, RT
	ldd		RT, y + 2
	eor		\i + 10, RT

	dec r29
.endm

.macro MULL_STEPF i
	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldi		r29, hi8(fb_muln_table0)

	MUL_STEP \i, 0

	ldd		RT, C_PTR + 24 + \i
	eor		\i, RT
	std		C_PTR + 24 + \i, \i
.endm

.macro MULN_DO
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23
		MULH \i
	.endr

	sbiw	r26, 24
	ldi		r29, hi8(fb_muln_table1)

	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12
		MULH_STEP10	\i
	.endr

	adiw	r26, 11
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9
		MULH_STEPF	\i
	.endr

	sbiw	r26, 21
	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		13, RT
	ldd		RT, y + 9
	eor		13 + 1, RT
	ldd		RT, y + 10
	eor		13 + 2, RT
	ldd		RT, y + 11
	eor		13 + 3, RT
	ldd		RT, y + 12
	eor		13 + 4, RT
	ldd		RT, y + 13
	eor		13 + 5, RT
	ldd		RT, y + 14
	eor		13 + 6, RT
	ldd		RT, y + 15
	eor		13 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		13 + 8, RT
	ldd		RT, y + 1
	eor		13 + 9, RT
	ldd		RT, y + 2
	eor		23, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		14, RT
	ldd		RT, y + 9
	eor		14 + 1, RT
	ldd		RT, y + 10
	eor		14 + 2, RT
	ldd		RT, y + 11
	eor		14 + 3, RT
	ldd		RT, y + 12
	eor		14 + 4, RT
	ldd		RT, y + 13
	eor		14 + 5, RT
	ldd		RT, y + 14
	eor		14 + 6, RT
	ldd		RT, y + 15
	eor		14 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		14 + 8, RT
	ldd		RT, y + 1
	eor		14 + 9, RT
	ldd		RT, y + 2
	eor		0, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		15, RT
	ldd		RT, y + 9
	eor		15 + 1, RT
	ldd		RT, y + 10
	eor		15 + 2, RT
	ldd		RT, y + 11
	eor		15 + 3, RT
	ldd		RT, y + 12
	eor		15 + 4, RT
	ldd		RT, y + 13
	eor		15 + 5, RT
	ldd		RT, y + 14
	eor		15 + 6, RT
	ldd		RT, y + 15
	eor		15 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		15 + 8, RT
	ldd		RT, y + 1
	eor		0, RT
	ldd		RT, y + 2
	eor		r1, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		16, RT
	ldd		RT, y + 9
	eor		16 + 1, RT
	ldd		RT, y + 10
	eor		16 + 2, RT
	ldd		RT, y + 11
	eor		16 + 3, RT
	ldd		RT, y + 12
	eor		16 + 4, RT
	ldd		RT, y + 13
	eor		16 + 5, RT
	ldd		RT, y + 14
	eor		16 + 6, RT
	ldd		RT, y + 15
	eor		16 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		0, RT
	ldd		RT, y + 1
	eor		1, RT
	ldd		RT, y + 2
	eor		2, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		17, RT
	ldd		RT, y + 9
	eor		17 + 1, RT
	ldd		RT, y + 10
	eor		17 + 2, RT
	ldd		RT, y + 11
	eor		17 + 3, RT
	ldd		RT, y + 12
	eor		17 + 4, RT
	ldd		RT, y + 13
	eor		17 + 5, RT
	ldd		RT, y + 14
	eor		17 + 6, RT
	ldd		RT, y + 15
	eor		0, RT
	inc r29
	ldd		RT, y + 0
	eor		1, RT
	ldd		RT, y + 1
	eor		2, RT
	ldd		RT, y + 2
	eor		3, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		18, RT
	ldd		RT, y + 9
	eor		18 + 1, RT
	ldd		RT, y + 10
	eor		18 + 2, RT
	ldd		RT, y + 11
	eor		18 + 3, RT
	ldd		RT, y + 12
	eor		18 + 4, RT
	ldd		RT, y + 13
	eor		18 + 5, RT
	ldd		RT, y + 14
	eor		0, RT
	ldd		RT, y + 15
	eor		1, RT
	inc r29
	ldd		RT, y + 0
	eor		2, RT
	ldd		RT, y + 1
	eor		3, RT
	ldd		RT, y + 2
	eor		r4, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		19, RT
	ldd		RT, y + 9
	eor		19 + 1, RT
	ldd		RT, y + 10
	eor		19 + 2, RT
	ldd		RT, y + 11
	eor		19 + 3, RT
	ldd		RT, y + 12
	eor		19 + 4, RT
	ldd		RT, y + 13
	eor		0, RT
	ldd		RT, y + 14
	eor		1, RT
	ldd		RT, y + 15
	eor		2, RT
	inc r29
	ldd		RT, y + 0
	eor		3, RT
	ldd		RT, y + 1
	eor		4, RT
	ldd		RT, y + 2
	eor		r5, RT
	dec r29

	clr		r9
	.irp i, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19
		swap	\i
		mov		RT, \i
		andi	RT, 0x0F
		eor		\i, RT
		eor		\i, RC
		mov		RC, RT

		std		C_PTR + 24 + \i, \i
	.endr

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r20, RT
	ldd		RT, y + 9
	eor		r21, RT
	ldd		RT, y + 10
	eor		r22, RT
	ldd		RT, y + 11
	eor		r23, RT
	ldd		RT, y + 12
	eor		r0, RT
	ldd		RT, y + 13
	eor		r1, RT
	ldd		RT, y + 14
	eor		r2, RT
	ldd		RT, y + 15
	eor		r3, RT
	inc r29
	ldd		RT, y + 0
	eor		r4, RT
	ldd		RT, y + 1
	eor		r5, RT
	ldd		RT, y + 2
	eor		r6, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r21, RT
	ldd		RT, y + 9
	eor		r22, RT
	ldd		RT, y + 10
	eor		r23, RT
	ldd		RT, y + 11
	eor		r0, RT
	ldd		RT, y + 12
	eor		r1, RT
	ldd		RT, y + 13
	eor		r2, RT
	ldd		RT, y + 14
	eor		r3, RT
	ldd		RT, y + 15
	eor		r4, RT
	inc r29
	ldd		RT, y + 0
	eor		r5, RT
	ldd		RT, y + 1
	eor		r6, RT
	ldd		RT, y + 2
	eor		r7, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r22, RT
	ldd		RT, y + 9
	eor		r23, RT
	ldd		RT, y + 10
	eor		r0, RT
	ldd		RT, y + 11
	eor		r1, RT
	ldd		RT, y + 12
	eor		r2, RT
	ldd		RT, y + 13
	eor		r3, RT
	ldd		RT, y + 14
	eor		r4, RT
	ldd		RT, y + 15
	eor		r5, RT
	inc r29
	ldd		RT, y + 0
	eor		r6, RT
	ldd		RT, y + 1
	eor		r7, RT
	ldd		RT, y + 2
	eor		r8, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r23, RT
	ldd		RT, y + 9
	eor		r0, RT
	ldd		RT, y + 10
	eor		r1, RT
	ldd		RT, y + 11
	eor		r2, RT
	ldd		RT, y + 12
	eor		r3, RT
	ldd		RT, y + 13
	eor		r4, RT
	ldd		RT, y + 14
	eor		r5, RT
	ldd		RT, y + 15
	eor		r6, RT
	inc r29
	ldd		RT, y + 0
	eor		r7, RT
	ldd		RT, y + 1
	eor		r8, RT
	ldd		r9, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r0, RT
	ldd		RT, y + 9
	eor		r1, RT
	ldd		RT, y + 10
	eor		r2, RT
	ldd		RT, y + 11
	eor		r3, RT
	ldd		RT, y + 12
	eor		r4, RT
	ldd		RT, y + 13
	eor		r5, RT
	ldd		RT, y + 14
	eor		r6, RT
	ldd		RT, y + 15
	eor		r7, RT
	inc r29
	ldd		RT, y + 0
	eor		r8, RT
	ldd		RT, y + 1
	eor		r9, RT
	ldd		r10, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r1, RT
	ldd		RT, y + 9
	eor		r2, RT
	ldd		RT, y + 10
	eor		r3, RT
	ldd		RT, y + 11
	eor		r4, RT
	ldd		RT, y + 12
	eor		r5, RT
	ldd		RT, y + 13
	eor		r6, RT
	ldd		RT, y + 14
	eor		r7, RT
	ldd		RT, y + 15
	eor		r8, RT
	inc r29
	ldd		RT, y + 0
	eor		r9, RT
	ldd		RT, y + 1
	eor		r10, RT
	ldd		r11, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r2, RT
	ldd		RT, y + 9
	eor		r3, RT
	ldd		RT, y + 10
	eor		r4, RT
	ldd		RT, y + 11
	eor		r5, RT
	ldd		RT, y + 12
	eor		r6, RT
	ldd		RT, y + 13
	eor		r7, RT
	ldd		RT, y + 14
	eor		r8, RT
	ldd		RT, y + 15
	eor		r9, RT
	inc r29
	ldd		RT, y + 0
	eor		r10, RT
	ldd		RT, y + 1
	eor		r11, RT
	ldd		r12, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r3, RT
	ldd		RT, y + 9
	eor		r4, RT
	ldd		RT, y + 10
	eor		r5, RT
	ldd		RT, y + 11
	eor		r6, RT
	ldd		RT, y + 12
	eor		r7, RT
	ldd		RT, y + 13
	eor		r8, RT
	ldd		RT, y + 14
	eor		r9, RT
	ldd		RT, y + 15
	eor		r10, RT
	inc r29
	ldd		RT, y + 0
	eor		r11, RT
	ldd		RT, y + 1
	eor		r12, RT
	ldd		r13, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r4, RT
	ldd		RT, y + 9
	eor		r5, RT
	ldd		RT, y + 10
	eor		r6, RT
	ldd		RT, y + 11
	eor		r7, RT
	ldd		RT, y + 12
	eor		r8, RT
	ldd		RT, y + 13
	eor		r9, RT
	ldd		RT, y + 14
	eor		r10, RT
	ldd		RT, y + 15
	eor		r11, RT
	inc r29
	ldd		RT, y + 0
	eor		r12, RT
	ldd		RT, y + 1
	eor		r13, RT
	ldd		r14, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r5, RT
	ldd		RT, y + 9
	eor		r6, RT
	ldd		RT, y + 10
	eor		r7, RT
	ldd		RT, y + 11
	eor		r8, RT
	ldd		RT, y + 12
	eor		r9, RT
	ldd		RT, y + 13
	eor		r10, RT
	ldd		RT, y + 14
	eor		r11, RT
	ldd		RT, y + 15
	eor		r12, RT
	inc r29
	ldd		RT, y + 0
	eor		r13, RT
	ldd		RT, y + 1
	eor		r14, RT
	ldd		r15, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r6, RT
	ldd		RT, y + 9
	eor		r7, RT
	ldd		RT, y + 10
	eor		r8, RT
	ldd		RT, y + 11
	eor		r9, RT
	ldd		RT, y + 12
	eor		r10, RT
	ldd		RT, y + 13
	eor		r11, RT
	ldd		RT, y + 14
	eor		r12, RT
	ldd		RT, y + 15
	eor		r13, RT
	inc r29
	ldd		RT, y + 0
	eor		r14, RT
	ldd		RT, y + 1
	eor		r15, RT
	ldd		r16, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r7, RT
	ldd		RT, y + 9
	eor		r8, RT
	ldd		RT, y + 10
	eor		r9, RT
	ldd		RT, y + 11
	eor		r10, RT
	ldd		RT, y + 12
	eor		r11, RT
	ldd		RT, y + 13
	eor		r12, RT
	ldd		RT, y + 14
	eor		r13, RT
	ldd		RT, y + 15
	eor		r14, RT
	inc r29
	ldd		RT, y + 0
	eor		r15, RT
	ldd		RT, y + 1
	eor		r16, RT
	ldd		r17, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r8, RT
	ldd		RT, y + 9
	eor		r9, RT
	ldd		RT, y + 10
	eor		r10, RT
	ldd		RT, y + 11
	eor		r11, RT
	ldd		RT, y + 12
	eor		r12, RT
	ldd		RT, y + 13
	eor		r13, RT
	ldd		RT, y + 14
	eor		r14, RT
	ldd		RT, y + 15
	eor		r15, RT
	inc r29
	ldd		RT, y + 0
	eor		r16, RT
	ldd		RT, y + 1
	eor		r17, RT
	ldd		r18, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0xF0
	ldd		RT, y + 8
	eor		r9, RT
	ldd		RT, y + 9
	eor		r10, RT
	ldd		RT, y + 10
	eor		r11, RT
	ldd		RT, y + 11
	eor		r12, RT
	ldd		RT, y + 12
	eor		r13, RT
	ldd		RT, y + 13
	eor		r14, RT
	ldd		RT, y + 14
	eor		r15, RT
	ldd		RT, y + 15
	eor		r16, RT
	inc r29
	ldd		RT, y + 0
	eor		r17, RT
	ldd		RT, y + 1
	eor		r18, RT
	ldd		r19, y + 2
	dec r29

	swap	r19
	mov		RT, r19
	andi	RT, 0x0F
	eor		r19, RT

	adiw	r30, 48
	.irp i, 18, 17, 16, 15
		SHIFT	\i, 0
	.endr
	sbiw	r30, 48

	.irp i, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0
		SHIFT	\i, 48
	.endr

	swap	r23
	mov		RT, r23
	andi	RT, 0x0F
	eor		r23, RT
	eor		r0, RT
	std 	C_PTR + 48, r0

	.irp i, 22, 21, 20
		SHIFT	\i, 24
	.endr

	eor		r20, RC
	std		C_PTR + 44, r20

	sbiw	r26, 34		; x = &a

	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23
		MULL \i
	.endr

	sbiw	r26, 24
	ldi		r29, hi8(fb_muln_table1)

	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12
		MULL_STEP10	\i
	.endr

	adiw	r26, 11
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9
		MULL_STEPF	\i
	.endr

	sbiw	r26, 21
	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		13, RT
	ldd		RT, y + 9
	eor		13 + 1, RT
	ldd		RT, y + 10
	eor		13 + 2, RT
	ldd		RT, y + 11
	eor		13 + 3, RT
	ldd		RT, y + 12
	eor		13 + 4, RT
	ldd		RT, y + 13
	eor		13 + 5, RT
	ldd		RT, y + 14
	eor		13 + 6, RT
	ldd		RT, y + 15
	eor		13 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		13 + 8, RT
	ldd		RT, y + 1
	eor		13 + 9, RT
	ldd		RT, y + 2
	eor		23, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		14, RT
	ldd		RT, y + 9
	eor		14 + 1, RT
	ldd		RT, y + 10
	eor		14 + 2, RT
	ldd		RT, y + 11
	eor		14 + 3, RT
	ldd		RT, y + 12
	eor		14 + 4, RT
	ldd		RT, y + 13
	eor		14 + 5, RT
	ldd		RT, y + 14
	eor		14 + 6, RT
	ldd		RT, y + 15
	eor		14 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		14 + 8, RT
	ldd		RT, y + 1
	eor		14 + 9, RT
	ldd		RT, y + 2
	eor		0, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		15, RT
	ldd		RT, y + 9
	eor		15 + 1, RT
	ldd		RT, y + 10
	eor		15 + 2, RT
	ldd		RT, y + 11
	eor		15 + 3, RT
	ldd		RT, y + 12
	eor		15 + 4, RT
	ldd		RT, y + 13
	eor		15 + 5, RT
	ldd		RT, y + 14
	eor		15 + 6, RT
	ldd		RT, y + 15
	eor		15 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		15 + 8, RT
	ldd		RT, y + 1
	eor		0, RT
	ldd		RT, y + 2
	eor		r1, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		16, RT
	ldd		RT, y + 9
	eor		16 + 1, RT
	ldd		RT, y + 10
	eor		16 + 2, RT
	ldd		RT, y + 11
	eor		16 + 3, RT
	ldd		RT, y + 12
	eor		16 + 4, RT
	ldd		RT, y + 13
	eor		16 + 5, RT
	ldd		RT, y + 14
	eor		16 + 6, RT
	ldd		RT, y + 15
	eor		16 + 7, RT
	inc r29
	ldd		RT, y + 0
	eor		0, RT
	ldd		RT, y + 1
	eor		1, RT
	ldd		RT, y + 2
	eor		2, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		17, RT
	ldd		RT, y + 9
	eor		17 + 1, RT
	ldd		RT, y + 10
	eor		17 + 2, RT
	ldd		RT, y + 11
	eor		17 + 3, RT
	ldd		RT, y + 12
	eor		17 + 4, RT
	ldd		RT, y + 13
	eor		17 + 5, RT
	ldd		RT, y + 14
	eor		17 + 6, RT
	ldd		RT, y + 15
	eor		0, RT
	inc r29
	ldd		RT, y + 0
	eor		1, RT
	ldd		RT, y + 1
	eor		2, RT
	ldd		RT, y + 2
	eor		3, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		18, RT
	ldd		RT, y + 9
	eor		18 + 1, RT
	ldd		RT, y + 10
	eor		18 + 2, RT
	ldd		RT, y + 11
	eor		18 + 3, RT
	ldd		RT, y + 12
	eor		18 + 4, RT
	ldd		RT, y + 13
	eor		18 + 5, RT
	ldd		RT, y + 14
	eor		0, RT
	ldd		RT, y + 15
	eor		1, RT
	inc r29
	ldd		RT, y + 0
	eor		2, RT
	ldd		RT, y + 1
	eor		3, RT
	ldd		RT, y + 2
	eor		r4, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		19, RT
	ldd		RT, y + 9
	eor		19 + 1, RT
	ldd		RT, y + 10
	eor		19 + 2, RT
	ldd		RT, y + 11
	eor		19 + 3, RT
	ldd		RT, y + 12
	eor		19 + 4, RT
	ldd		RT, y + 13
	eor		0, RT
	ldd		RT, y + 14
	eor		1, RT
	ldd		RT, y + 15
	eor		2, RT
	inc r29
	ldd		RT, y + 0
	eor		3, RT
	ldd		RT, y + 1
	eor		4, RT
	ldd		RT, y + 2
	eor		r5, RT
	dec r29

	clr		r9
	.irp i, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19
		ldd		RT, C_PTR + 24 + \i
		eor		\i, RT
		std		C_PTR + 24 + \i, \i
	.endr

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r20, RT
	ldd		RT, y + 9
	eor		r21, RT
	ldd		RT, y + 10
	eor		r22, RT
	ldd		RT, y + 11
	eor		r23, RT
	ldd		RT, y + 12
	eor		r0, RT
	ldd		RT, y + 13
	eor		r1, RT
	ldd		RT, y + 14
	eor		r2, RT
	ldd		RT, y + 15
	eor		r3, RT
	inc r29
	ldd		RT, y + 0
	eor		r4, RT
	ldd		RT, y + 1
	eor		r5, RT
	ldd		RT, y + 2
	eor		r6, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r21, RT
	ldd		RT, y + 9
	eor		r22, RT
	ldd		RT, y + 10
	eor		r23, RT
	ldd		RT, y + 11
	eor		r0, RT
	ldd		RT, y + 12
	eor		r1, RT
	ldd		RT, y + 13
	eor		r2, RT
	ldd		RT, y + 14
	eor		r3, RT
	ldd		RT, y + 15
	eor		r4, RT
	inc r29
	ldd		RT, y + 0
	eor		r5, RT
	ldd		RT, y + 1
	eor		r6, RT
	ldd		RT, y + 2
	eor		r7, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r22, RT
	ldd		RT, y + 9
	eor		r23, RT
	ldd		RT, y + 10
	eor		r0, RT
	ldd		RT, y + 11
	eor		r1, RT
	ldd		RT, y + 12
	eor		r2, RT
	ldd		RT, y + 13
	eor		r3, RT
	ldd		RT, y + 14
	eor		r4, RT
	ldd		RT, y + 15
	eor		r5, RT
	inc r29
	ldd		RT, y + 0
	eor		r6, RT
	ldd		RT, y + 1
	eor		r7, RT
	ldd		RT, y + 2
	eor		r8, RT
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r23, RT
	ldd		RT, y + 9
	eor		r0, RT
	ldd		RT, y + 10
	eor		r1, RT
	ldd		RT, y + 11
	eor		r2, RT
	ldd		RT, y + 12
	eor		r3, RT
	ldd		RT, y + 13
	eor		r4, RT
	ldd		RT, y + 14
	eor		r5, RT
	ldd		RT, y + 15
	eor		r6, RT
	inc r29
	ldd		RT, y + 0
	eor		r7, RT
	ldd		RT, y + 1
	eor		r8, RT
	ldd		r9, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r0, RT
	ldd		RT, y + 9
	eor		r1, RT
	ldd		RT, y + 10
	eor		r2, RT
	ldd		RT, y + 11
	eor		r3, RT
	ldd		RT, y + 12
	eor		r4, RT
	ldd		RT, y + 13
	eor		r5, RT
	ldd		RT, y + 14
	eor		r6, RT
	ldd		RT, y + 15
	eor		r7, RT
	inc r29
	ldd		RT, y + 0
	eor		r8, RT
	ldd		RT, y + 1
	eor		r9, RT
	ldd		r10, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r1, RT
	ldd		RT, y + 9
	eor		r2, RT
	ldd		RT, y + 10
	eor		r3, RT
	ldd		RT, y + 11
	eor		r4, RT
	ldd		RT, y + 12
	eor		r5, RT
	ldd		RT, y + 13
	eor		r6, RT
	ldd		RT, y + 14
	eor		r7, RT
	ldd		RT, y + 15
	eor		r8, RT
	inc r29
	ldd		RT, y + 0
	eor		r9, RT
	ldd		RT, y + 1
	eor		r10, RT
	ldd		r11, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r2, RT
	ldd		RT, y + 9
	eor		r3, RT
	ldd		RT, y + 10
	eor		r4, RT
	ldd		RT, y + 11
	eor		r5, RT
	ldd		RT, y + 12
	eor		r6, RT
	ldd		RT, y + 13
	eor		r7, RT
	ldd		RT, y + 14
	eor		r8, RT
	ldd		RT, y + 15
	eor		r9, RT
	inc r29
	ldd		RT, y + 0
	eor		r10, RT
	ldd		RT, y + 1
	eor		r11, RT
	ldd		r12, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r3, RT
	ldd		RT, y + 9
	eor		r4, RT
	ldd		RT, y + 10
	eor		r5, RT
	ldd		RT, y + 11
	eor		r6, RT
	ldd		RT, y + 12
	eor		r7, RT
	ldd		RT, y + 13
	eor		r8, RT
	ldd		RT, y + 14
	eor		r9, RT
	ldd		RT, y + 15
	eor		r10, RT
	inc r29
	ldd		RT, y + 0
	eor		r11, RT
	ldd		RT, y + 1
	eor		r12, RT
	ldd		r13, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r4, RT
	ldd		RT, y + 9
	eor		r5, RT
	ldd		RT, y + 10
	eor		r6, RT
	ldd		RT, y + 11
	eor		r7, RT
	ldd		RT, y + 12
	eor		r8, RT
	ldd		RT, y + 13
	eor		r9, RT
	ldd		RT, y + 14
	eor		r10, RT
	ldd		RT, y + 15
	eor		r11, RT
	inc r29
	ldd		RT, y + 0
	eor		r12, RT
	ldd		RT, y + 1
	eor		r13, RT
	ldd		r14, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r5, RT
	ldd		RT, y + 9
	eor		r6, RT
	ldd		RT, y + 10
	eor		r7, RT
	ldd		RT, y + 11
	eor		r8, RT
	ldd		RT, y + 12
	eor		r9, RT
	ldd		RT, y + 13
	eor		r10, RT
	ldd		RT, y + 14
	eor		r11, RT
	ldd		RT, y + 15
	eor		r12, RT
	inc r29
	ldd		RT, y + 0
	eor		r13, RT
	ldd		RT, y + 1
	eor		r14, RT
	ldd		r15, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r6, RT
	ldd		RT, y + 9
	eor		r7, RT
	ldd		RT, y + 10
	eor		r8, RT
	ldd		RT, y + 11
	eor		r9, RT
	ldd		RT, y + 12
	eor		r10, RT
	ldd		RT, y + 13
	eor		r11, RT
	ldd		RT, y + 14
	eor		r12, RT
	ldd		RT, y + 15
	eor		r13, RT
	inc r29
	ldd		RT, y + 0
	eor		r14, RT
	ldd		RT, y + 1
	eor		r15, RT
	ldd		r16, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r7, RT
	ldd		RT, y + 9
	eor		r8, RT
	ldd		RT, y + 10
	eor		r9, RT
	ldd		RT, y + 11
	eor		r10, RT
	ldd		RT, y + 12
	eor		r11, RT
	ldd		RT, y + 13
	eor		r12, RT
	ldd		RT, y + 14
	eor		r13, RT
	ldd		RT, y + 15
	eor		r14, RT
	inc r29
	ldd		RT, y + 0
	eor		r15, RT
	ldd		RT, y + 1
	eor		r16, RT
	ldd		r17, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r8, RT
	ldd		RT, y + 9
	eor		r9, RT
	ldd		RT, y + 10
	eor		r10, RT
	ldd		RT, y + 11
	eor		r11, RT
	ldd		RT, y + 12
	eor		r12, RT
	ldd		RT, y + 13
	eor		r13, RT
	ldd		RT, y + 14
	eor		r14, RT
	ldd		RT, y + 15
	eor		r15, RT
	inc r29
	ldd		RT, y + 0
	eor		r16, RT
	ldd		RT, y + 1
	eor		r17, RT
	ldd		r18, y + 2
	dec r29

	ld 		r28, A_PTR+
	andi	r28, 0x0F
	swap	r28
	ldd		RT, y + 8
	eor		r9, RT
	ldd		RT, y + 9
	eor		r10, RT
	ldd		RT, y + 10
	eor		r11, RT
	ldd		RT, y + 11
	eor		r12, RT
	ldd		RT, y + 12
	eor		r13, RT
	ldd		RT, y + 13
	eor		r14, RT
	ldd		RT, y + 14
	eor		r15, RT
	ldd		RT, y + 15
	eor		r16, RT
	inc r29
	ldd		RT, y + 0
	eor		r17, RT
	ldd		RT, y + 1
	eor		r18, RT
	ldd		r19, y + 2
	dec r29

.endm

fb_muln_low:
	PROLOGUE

	; C is stored on r25:r24
	movw	r26, r22		; copy a to x
	movw	r30, r20		; copy b to z
	movw	r22, r24

	MULN_TABLE

	movw	r30, r22		; z = &c

	MULN_DO

	adiw	r30, 48
	.irp	i, 19, 18, 17, 16
		ldd		RT, C_PTR + \i
		eor		RT, \i
		std		C_PTR + \i, RT
	.endr
	sbiw	r30, 48

	.irp	i, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0
		ldd		RT, C_PTR + 48 + \i
		eor		RT, \i
		std		C_PTR + 48 + \i, RT
	.endr

	.irp i, 23, 22, 21, 20
		ldd		RT, C_PTR + 24 + \i
		eor		RT, \i
		std		C_PTR + 24 + \i, RT
	.endr

	EPILOGUE
