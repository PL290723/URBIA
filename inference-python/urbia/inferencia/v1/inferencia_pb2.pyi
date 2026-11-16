from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Resultado(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RESULTADO_NO_ESPECIFICADO: _ClassVar[Resultado]
    RESULTADO_DETECTADO: _ClassVar[Resultado]
    RESULTADO_NO_DETECTADO: _ClassVar[Resultado]
RESULTADO_NO_ESPECIFICADO: Resultado
RESULTADO_DETECTADO: Resultado
RESULTADO_NO_DETECTADO: Resultado

class DetectarRequest(_message.Message):
    __slots__ = ("id_solicitud", "id_reporte", "id_archivo", "ruta_archivo", "opciones")
    ID_SOLICITUD_FIELD_NUMBER: _ClassVar[int]
    ID_REPORTE_FIELD_NUMBER: _ClassVar[int]
    ID_ARCHIVO_FIELD_NUMBER: _ClassVar[int]
    RUTA_ARCHIVO_FIELD_NUMBER: _ClassVar[int]
    OPCIONES_FIELD_NUMBER: _ClassVar[int]
    id_solicitud: str
    id_reporte: int
    id_archivo: int
    ruta_archivo: str
    opciones: OpcionesDeteccion
    def __init__(self, id_solicitud: _Optional[str] = ..., id_reporte: _Optional[int] = ..., id_archivo: _Optional[int] = ..., ruta_archivo: _Optional[str] = ..., opciones: _Optional[_Union[OpcionesDeteccion, _Mapping]] = ...) -> None: ...

class OpcionesDeteccion(_message.Message):
    __slots__ = ("omitir_cajas", "max_cajas")
    OMITIR_CAJAS_FIELD_NUMBER: _ClassVar[int]
    MAX_CAJAS_FIELD_NUMBER: _ClassVar[int]
    omitir_cajas: bool
    max_cajas: int
    def __init__(self, omitir_cajas: _Optional[bool] = ..., max_cajas: _Optional[int] = ...) -> None: ...

class Caja(_message.Message):
    __slots__ = ("clase", "confianza", "x1", "y1", "x2", "y2")
    CLASE_FIELD_NUMBER: _ClassVar[int]
    CONFIANZA_FIELD_NUMBER: _ClassVar[int]
    X1_FIELD_NUMBER: _ClassVar[int]
    Y1_FIELD_NUMBER: _ClassVar[int]
    X2_FIELD_NUMBER: _ClassVar[int]
    Y2_FIELD_NUMBER: _ClassVar[int]
    clase: str
    confianza: float
    x1: float
    y1: float
    x2: float
    y2: float
    def __init__(self, clase: _Optional[str] = ..., confianza: _Optional[float] = ..., x1: _Optional[float] = ..., y1: _Optional[float] = ..., x2: _Optional[float] = ..., y2: _Optional[float] = ...) -> None: ...

class ModeloUsado(_message.Message):
    __slots__ = ("nombre", "version", "umbral_confianza")
    NOMBRE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    UMBRAL_CONFIANZA_FIELD_NUMBER: _ClassVar[int]
    nombre: str
    version: str
    umbral_confianza: float
    def __init__(self, nombre: _Optional[str] = ..., version: _Optional[str] = ..., umbral_confianza: _Optional[float] = ...) -> None: ...

class DetectarResponse(_message.Message):
    __slots__ = ("id_solicitud", "resultado", "clase_detectada", "confianza", "cajas", "modelo", "ancho", "alto", "milisegundos")
    ID_SOLICITUD_FIELD_NUMBER: _ClassVar[int]
    RESULTADO_FIELD_NUMBER: _ClassVar[int]
    CLASE_DETECTADA_FIELD_NUMBER: _ClassVar[int]
    CONFIANZA_FIELD_NUMBER: _ClassVar[int]
    CAJAS_FIELD_NUMBER: _ClassVar[int]
    MODELO_FIELD_NUMBER: _ClassVar[int]
    ANCHO_FIELD_NUMBER: _ClassVar[int]
    ALTO_FIELD_NUMBER: _ClassVar[int]
    MILISEGUNDOS_FIELD_NUMBER: _ClassVar[int]
    id_solicitud: str
    resultado: Resultado
    clase_detectada: str
    confianza: float
    cajas: _containers.RepeatedCompositeFieldContainer[Caja]
    modelo: ModeloUsado
    ancho: int
    alto: int
    milisegundos: float
    def __init__(self, id_solicitud: _Optional[str] = ..., resultado: _Optional[_Union[Resultado, str]] = ..., clase_detectada: _Optional[str] = ..., confianza: _Optional[float] = ..., cajas: _Optional[_Iterable[_Union[Caja, _Mapping]]] = ..., modelo: _Optional[_Union[ModeloUsado, _Mapping]] = ..., ancho: _Optional[int] = ..., alto: _Optional[int] = ..., milisegundos: _Optional[float] = ...) -> None: ...

class ObtenerInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ObtenerInfoResponse(_message.Message):
    __slots__ = ("version_contrato", "modelo", "clases", "imgsz", "dispositivo", "sha256_pesos", "peticiones", "errores")
    VERSION_CONTRATO_FIELD_NUMBER: _ClassVar[int]
    MODELO_FIELD_NUMBER: _ClassVar[int]
    CLASES_FIELD_NUMBER: _ClassVar[int]
    IMGSZ_FIELD_NUMBER: _ClassVar[int]
    DISPOSITIVO_FIELD_NUMBER: _ClassVar[int]
    SHA256_PESOS_FIELD_NUMBER: _ClassVar[int]
    PETICIONES_FIELD_NUMBER: _ClassVar[int]
    ERRORES_FIELD_NUMBER: _ClassVar[int]
    version_contrato: str
    modelo: ModeloUsado
    clases: _containers.RepeatedScalarFieldContainer[str]
    imgsz: int
    dispositivo: str
    sha256_pesos: str
    peticiones: int
    errores: int
    def __init__(self, version_contrato: _Optional[str] = ..., modelo: _Optional[_Union[ModeloUsado, _Mapping]] = ..., clases: _Optional[_Iterable[str]] = ..., imgsz: _Optional[int] = ..., dispositivo: _Optional[str] = ..., sha256_pesos: _Optional[str] = ..., peticiones: _Optional[int] = ..., errores: _Optional[int] = ...) -> None: ...
