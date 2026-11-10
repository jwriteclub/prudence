// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from TLA+ SyntaxTreeConstants.java, Copyright (c) 2003 Compaq
// Corporation, distributed under the MIT license. See NOTICE.
// Source SHA-256: cdc18e242fd10f149a8404e6dbb5d0358ed2b0c94716c2aba09e8a9aea36d27f
package generated

import "fmt"

const (
	NULL_ID                = 327
	N_ActDecl              = 328
	N_ActionExpr           = 329
	N_AssumeDecl           = 330
	N_AssumeProve          = 331
	N_Assumption           = 332
	N_BeginModule          = 333
	N_Body                 = 334
	N_BoundQuant           = 335
	N_Case                 = 336
	N_CaseArm              = 337
	N_ConjItem             = 340
	N_ConjList             = 341
	N_ConsDecl             = 342
	N_DisjItem             = 343
	N_DisjList             = 344
	N_EndModule            = 345
	N_Except               = 346
	N_ExceptComponent      = 347
	N_ExceptSpec           = 348
	N_Times                = 349
	N_Extends              = 350
	N_FairnessExpr         = 351
	N_FcnAppl              = 352
	N_FcnConst             = 353
	N_FieldSet             = 354
	N_FieldVal             = 355
	N_FunctionDefinition   = 356
	N_FunctionParam        = 357
	N_GeneralId            = 358
	N_GenInfixOp           = 359
	N_GenNonExpPrefixOp    = 360
	N_GenPostfixOp         = 361
	N_GenPrefixOp          = 362
	N_IdentDecl            = 363
	N_Real                 = 364
	N_IdentifierTuple      = 365
	N_IdentLHS             = 366
	N_IdPrefix             = 367
	N_IdPrefixElement      = 368
	N_IfThenElse           = 369
	N_InfixDecl            = 370
	N_InfixExpr            = 371
	N_InfixLHS             = 372
	N_InfixOp              = 373
	N_InnerProof           = 374
	N_Instance             = 375
	N_NonLocalInstance     = 376
	N_Integer              = 377
	N_LetDefinitions       = 379
	N_LetIn                = 380
	N_MaybeBound           = 381
	N_Module               = 382
	N_ModuleDefinition     = 383
	N_NonExpPrefixOp       = 384
	N_Number               = 385
	N_NumberedAssumeProve  = 386
	N_OpApplication        = 387
	N_OpArgs               = 388
	N_OperatorDefinition   = 389
	N_OtherArm             = 390
	N_ParamDecl            = 391
	N_ParamDeclaration     = 392
	N_ParenExpr            = 393
	N_PostfixDecl          = 394
	N_PostfixExpr          = 395
	N_PostfixLHS           = 396
	N_PostfixOp            = 397
	N_PrefixDecl           = 398
	N_PrefixExpr           = 399
	N_PrefixLHS            = 400
	N_PrefixOp             = 401
	N_Proof                = 402
	N_ProofStep            = 406
	N_QEDStep              = 407
	N_QuantBound           = 408
	N_RcdConstructor       = 409
	N_RecordComponent      = 410
	N_SetEnumerate         = 411
	N_SetOfAll             = 413
	N_SetOfFcns            = 414
	N_SetOfRcds            = 415
	N_String               = 418
	N_SubsetOf             = 419
	N_Substitution         = 420
	N_TempDecl             = 421
	N_Theorem              = 422
	N_Tuple                = 423
	N_UnboundOrBoundChoose = 424
	N_UnboundQuant         = 425
	N_VariableDeclaration  = 426
	T_IN                   = 427
	T_EQUAL                = 428
	N_NewSymb              = 429
	N_Lambda               = 430
	N_Recursive            = 431
	N_Label                = 432
	N_StructOp             = 433
	N_NumerableStep        = 434
	N_TerminalProof        = 435
	N_UseOrHide            = 436
	N_NonExprBody          = 437
	N_DefStep              = 438
	N_HaveStep             = 439
	N_TakeStep             = 440
	N_WitnessStep          = 441
	N_PickStep             = 442
	N_CaseStep             = 443
	N_AssertStep           = 444
)

var syntaxNodeImageStrings = [...]string{
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"Not a node",
	"N_ActDecl",
	"N_ActionExpr",
	"N_AssumeDecl",
	"N_AssumeProve",
	"N_Assumption",
	"N_BeginModule",
	"N_Body",
	"N_BoundedQuant",
	"N_Case",
	"N_CaseArm",
	"N_CaseStatement",
	"N_ChooseStatement",
	"N_ConjItem",
	"N_ConjList",
	"N_ConsDecl",
	"N_DisjItem",
	"N_DisjList",
	"N_EndModule",
	"N_Except",
	"N_ExceptComponent",
	"N_ExceptSpec",
	"N_Times",
	"N_Extends",
	"N_FairnessExpr",
	"N_FcnAppl",
	"N_FcnConst",
	"N_FieldSet",
	"N_FieldVal",
	"N_FunctionDefinition",
	"N_FunctionParam",
	"N_GeneralId",
	"N_GenInfixOp",
	"N_GenNonExpPrefixOp",
	"N_GenPostfixOp",
	"N_GenPrefixOp",
	"N_IdentDecl",
	"N_Real",
	"N_IdentifierTuple",
	"N_IdentLHS",
	"N_IdPrefix",
	"N_IdPrefixElement",
	"N_IfThenElse",
	"N_InfixDecl",
	"N_InfixExpr",
	"N_InfixLHS",
	"N_InfixOp",
	"N_InnerProof",
	"N_Instance",
	"N_NonLocalInstance",
	"N_Integer",
	"N_LeafProof",
	"N_LetDefinitions",
	"N_LetIn",
	"N_MaybeBound",
	"N_Module",
	"N_ModuleDefinition",
	"N_NonExpPrefixOp",
	"N_Number",
	"N_NumberedAssumeProve",
	"N_OpApplication",
	"N_OpArgs",
	"N_OperatorDefinition",
	"N_OtherArm",
	"N_ParamDecl",
	"N_ParamDeclaration",
	"N_ParenExpr",
	"N_PostfixDecl",
	"N_PostfixExpr",
	"N_PostfixLHS",
	"N_PostfixOp",
	"N_PrefixDecl",
	"N_PrefixExpr",
	"N_PrefixLHS",
	"N_PrefixOp",
	"N_Proof",
	"N_ProofLet",
	"N_ProofName",
	"N_ProofStatement",
	"N_ProofStep",
	"N_QEDStep",
	"N_QuantBound",
	"N_RcdConstructor",
	"N_RecordComponent",
	"N_SetEnumerate",
	"Not a node",
	"N_SetOfAll",
	"N_SetOfFcns",
	"N_SetOfRcds",
	"Not a node",
	"Not a node",
	"N_String",
	"N_SubsetOf",
	"N_Substitution",
	"N_TempDecl",
	"N_Theorem",
	"N_Tuple",
	"N_UnBoundedOrBoundedChoose",
	"N_UnboundedQuant",
	"N_VariableDeclaration",
	"Token =",
	"Token \\in",
	"N_NewSymb",
	"N_Lambda",
	"N_Recursive",
	"N_Label",
	"N_StructOp",
	"N_NumerableStep",
	"N_TerminalProof",
	"N_UseOrHide",
	"N_NonExprBody",
	"N_DefStep",
	"N_HaveStep",
	"N_TakeStep",
	"N_WitnessStep",
	"N_PickStep",
	"N_CaseStep",
	"N_AssertStep",
	"Not a node",
}

// SyntaxNodeImage returns the interned image for a Java table index.
func (ctx *SupportContext) SyntaxNodeImage(kind int) (*UniqueString, error) {
	if kind < 0 || kind >= len(syntaxNodeImageStrings) {
		return nil, fmt.Errorf("sanyparser: syntax node image index %d out of range", kind)
	}
	return ctx.UniqueStringOf(syntaxNodeImageStrings[kind]), nil
}
