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
/**
 * @file
 *
 * Binary field multiplication functions.
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
#define RT			r25

/*
 * Carry register for shifting.
 */
#define RC			r24

/*
 * Size in bytes of a precolow_kmputation table line.
 */
#define T_LINE 23

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
	.irp i, 0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 28, 29, 30, 31
		push 	\i
	.endr
.endm

.macro EPILOGUE
	.irp i, 31, 30, 29, 28, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 0
		pop 	\i
	.endr
	ret
.endm

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

.text

.global	fb_mulk_low
.global	fb_muln_low

.macro MULN_TABLE
	ldi		RC, 0x40
	ldi		r29, hi8(fb_muln_table0)
	FILL_TABLE 0, 15
	ldi		r29, hi8(fb_muln_table1)
	FILL_TABLE 16, 22
.endm

.macro MULN_DO
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21
		MULH \i
	.endr

	swap	r20
	mov		RT, r20
	andi	RT, 0x0F
	eor		r20, RT
	std		C_PTR + (20 + T_LINE + 1), RT

	.irp i, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0
		swap	\i
		mov		RT, \i
		andi	RT, 0x0F
		eor		\i, RT
		eor		\i + 1, RT
		std		C_PTR + (\i + T_LINE + 1), \i + 1
	.endr

	swap	r22
	mov		RT,r22
	andi	RT,0x0F
	eor		r22, RT
	eor		r0, RT
	std		C_PTR + 23, r0

	eor		r22, RC
	std		C_PTR + 22, r22

	sbiw	r26, 22		; x = &a

	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22
		MULL \i
	.endr
.endm

fb_mulk_low:
	.irp i, 16, 17, 28, 29, 30, 31
		push 	\i
	.endr

	; C is stored on r25:r24
	push 	r24
	push 	r25
	movw	r26, r22		; copy a to x
	movw	r30, r20		; copy b to z

	MULN_TABLE
	pop 	r31
	pop 	r30
	MULN_DO

	.irp i, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44
		ldd		RT, C_PTR + \i
		eor		RT, \i % T_LINE
		std		C_PTR + \i, RT
	.endr

	clr		r1
	std		C_PTR + 45, r1

	.irp i, 31, 30, 29, 28, 17, 16
		pop 	\i
	.endr

	ret

.data
.byte
mt0:.skip 45
.byte
mt1:.skip 45
.byte
a0:.skip 23
.byte
b0:.skip 23

.text

.macro	copy a b i j
	ldd		RT,\b+\i
	st		\a+,RT
.if \i-\j
	copy \a \b "(\i+1)" \j
.endif
.endm

.macro	copyxor a b i j
	ldd		RT,\b+\i
	ld		RC,\a
	eor		RT,RC
	st		\a+,RT
.if \i-\j
	copyxor \a \b "(\i+1)" \j
.endif
.endm

fb_muln_low:
	PROLOGUE

	movw	r16,r24				; r17:r16 = &c
	movw	r28,r22				; y = &a
	movw	r30,r20				; z = &b

	ldd		r0, y + 22
	push	r0
	std		y + 22, r1
	ldd		r0, z + 22
	push	r0
	std		z + 22, r1

	call fb_mulk_low			; c[0..45] = Mul(a[0..21], b[0..21]);

	pop		r0
	std		z + 22, r0
	pop		r0
	std		y + 22, r0

	movw	r24, r16
	adiw	r24, 44
	adiw	r28, 22
	movw	r22, r28
	adiw	r30, 22
	movw	r20, r30
	call fb_mulk_low			; c[44..89] = Mul(a[22..44], b[22..44]);
	sbiw	r28, 22
	sbiw	r30, 22

	ldi		r26,lo8(a0)
	ldi		r27,hi8(a0)			; a0 = a[0..21]^a[22..44]
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21
		ldd		RT, y + \i
		ldd		RC, y + \i + 22
		eor		RT, RC
		st		x+, RT
	.endr
	ldd		RT, y + 44
	st		x, RT

	ldi		r26,lo8(b0)
	ldi		r27,hi8(b0)			; b0 = b[0..21]^b[22..44]
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21
		ldd		RT, z + \i
		ldd		RC, z + \i + 22
		eor		RT, RC
		st		x+, RT
	.endr
	ldd		RT, z + 44
	st		x, RT

	movw	r28,r22				; y = &a
	movw	r30,r20				; z = &b

	push	r16
	push	r17

	movw	r26, r16
	adiw	r26, 22				; x = &m[22]
	movw	r28, r16
	movw	r30, r16
	adiw	r30, 44
	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21
		ld		\i,x+
		ldd		RT,y+\i
		eor		\i,RT
		ldd		RT,z+\i
		eor		\i,RT
	.endr

	.irp i, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43
		ld		RC,x
		ldd		RT,y+\i
		eor		RC,RT
		ldd		RT,z+\i
		eor		RC,RT
		st		x+,RC
	.endr
	ld		RC,x
	ldd		RT,z+44
	eor		RC,RT
	st		x,RC

	sbiw	r26, 44

	.irp i, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21
		st	x+, \i
	.endr

	pop	r17
	pop r16

	ldi		r24,lo8(mt0)
	ldi		r25,hi8(mt0)		; y = &a0
	ldi		r22,lo8(a0)
	ldi		r23,hi8(a0)
	ldi		r20,lo8(b0)
	ldi		r21,hi8(b0)
	call fb_mulk_low

	movw	r26,r16
	adiw	r26,22
	ldi		r28,lo8(mt0)
	ldi		r29,hi8(mt0)		; x = &m[10], y = &mt0

	copyxor x y 0 44			; m[22..66] ^= mt0[0..44]

	clr		r1
	EPILOGUE
